package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type trackingProvider struct {
	id      string
	started chan string
	release chan struct{}
}

func (p trackingProvider) ID() string                                           { return p.id }
func (p trackingProvider) Health(context.Context) error                         { return nil }
func (p trackingProvider) ListModels(context.Context) ([]provider.Model, error) { return nil, nil }
func (p trackingProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	if p.started != nil {
		p.started <- p.id
	}
	if p.release != nil {
		<-p.release
	}
	return provider.ChatResponse{Content: p.id}, nil
}
func (p trackingProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (p trackingProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: p.id}, nil
}

type memoryUsageStore struct {
	mu     sync.Mutex
	events []UsageEvent
}

func (s *memoryUsageStore) LoadExecutionUsage(_ context.Context, since time.Time) ([]UsageEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]UsageEvent, 0, len(s.events))
	for _, event := range s.events {
		if event.OccurredAt.After(since) {
			result = append(result, event)
		}
	}
	return result, nil
}

func (s *memoryUsageStore) RecordExecutionUsage(_ context.Context, event UsageEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

type fakeProvider struct {
	content string
	err     error
}

func (p fakeProvider) ID() string                                           { return "fake" }
func (p fakeProvider) Health(context.Context) error                         { return p.err }
func (p fakeProvider) ListModels(context.Context) ([]provider.Model, error) { return nil, p.err }
func (p fakeProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: p.content}, p.err
}
func (p fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return p.err
}
func (p fakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: p.content}, p.err
}
func (p fakeProvider) Embeddings(_ context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	return provider.EmbeddingResponse{Vectors: make([][]float64, len(req.Inputs))}, p.err
}
func (p fakeProvider) Rerank(_ context.Context, req provider.RerankRequest) (provider.RerankResponse, error) {
	return provider.RerankResponse{Results: []provider.RerankResult{{Index: len(req.Documents) - 1, Score: 0.9}}}, p.err
}

func TestExecuteFallsBackToSecondTarget(t *testing.T) {
	providers := map[string]provider.Provider{
		"first":  fakeProvider{err: errors.New("provider unavailable")},
		"second": fakeProvider{content: "ok"},
	}
	service := New([]config.ExecutionProfile{{
		ID: "quality", Capability: config.CapabilityText, Enabled: true, MaxConcurrency: 1,
		Targets: []config.ExecutionTarget{
			{ProviderID: "first", Enabled: true, Priority: 10},
			{ProviderID: "second", Model: "model-2", Enabled: true, Priority: 20},
		},
	}}, func(id string) (provider.Provider, bool) { value, ok := providers[id]; return value, ok })

	response, err := service.Execute(context.Background(), Request{Profile: "quality", Prompt: "hello"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if response.Content != "ok" || response.ProviderID != "second" || len(response.Attempts) != 2 {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestExecuteEmbeddingUsesEmbeddingCapability(t *testing.T) {
	service := New([]config.ExecutionProfile{{
		ID: "embedding", Capability: config.CapabilityEmbedding, Enabled: true, MaxConcurrency: 1,
		Targets: []config.ExecutionTarget{{ProviderID: "fake", Enabled: true}},
	}}, func(string) (provider.Provider, bool) { return fakeProvider{}, true })

	response, err := service.Execute(context.Background(), Request{Profile: "embedding", Inputs: []string{"a", "b"}})
	if err != nil || len(response.Vectors) != 2 {
		t.Fatalf("response = %#v, error = %v", response, err)
	}
}

func TestExecuteRejectsCapabilityMismatch(t *testing.T) {
	service := New([]config.ExecutionProfile{{
		ID: "text", Capability: config.CapabilityText, Enabled: true, MaxConcurrency: 1,
	}}, func(string) (provider.Provider, bool) { return fakeProvider{}, true })

	_, err := service.Execute(context.Background(), Request{Profile: "text", Capability: config.CapabilityVision})
	if !errors.Is(err, ErrCapability) {
		t.Fatalf("error = %v, want ErrCapability", err)
	}
}

func TestRoundRobinRotatesEqualPriorityTargets(t *testing.T) {
	providers := map[string]provider.Provider{
		"first":  trackingProvider{id: "first"},
		"second": trackingProvider{id: "second"},
	}
	service := New([]config.ExecutionProfile{{
		ID: "rotating", Capability: config.CapabilityText, SelectionStrategy: config.SelectionRoundRobin,
		Enabled: true, MaxConcurrency: 1,
		Targets: []config.ExecutionTarget{
			{ProviderID: "first", Enabled: true, Priority: 10},
			{ProviderID: "second", Enabled: true, Priority: 10},
		},
	}}, func(id string) (provider.Provider, bool) { value, ok := providers[id]; return value, ok })

	first, err := service.Execute(context.Background(), Request{Profile: "rotating", Prompt: "one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Execute(context.Background(), Request{Profile: "rotating", Prompt: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ProviderID != "first" || second.ProviderID != "second" {
		t.Fatalf("providers = %s, %s; want first, second", first.ProviderID, second.ProviderID)
	}
}

func TestRoundRobinLeasesSeparateKeysForConcurrentCalls(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{}, 2)
	providers := map[string]provider.Provider{
		"first":  trackingProvider{id: "first", started: started, release: release},
		"second": trackingProvider{id: "second", started: started, release: release},
	}
	service := New([]config.ExecutionProfile{{
		ID: "parallel", Capability: config.CapabilityText, SelectionStrategy: config.SelectionRoundRobin,
		Enabled: true, MaxConcurrency: 2, TimeoutSeconds: 5,
		Targets: []config.ExecutionTarget{
			{ProviderID: "first", Enabled: true, Priority: 10, MaxConcurrency: 1},
			{ProviderID: "second", Enabled: true, Priority: 10, MaxConcurrency: 1},
		},
	}}, func(id string) (provider.Provider, bool) { value, ok := providers[id]; return value, ok })

	errors := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := service.Execute(context.Background(), Request{Profile: "parallel", Prompt: "work"})
			errors <- err
		}()
	}
	seen := map[string]bool{<-started: true, <-started: true}
	if !seen["first"] || !seen["second"] {
		t.Fatalf("concurrent providers = %#v; want one lease per provider", seen)
	}
	release <- struct{}{}
	release <- struct{}{}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
	if err := <-errors; err != nil {
		t.Fatal(err)
	}
}

func TestDailyRateLimitBlocksBeforeProviderCallAndPersists(t *testing.T) {
	store := &memoryUsageStore{}
	providerCalls := make(chan string, 2)
	profile := config.ExecutionProfile{
		ID: "free", Capability: config.CapabilityText, Enabled: true, MaxConcurrency: 1,
		Targets: []config.ExecutionTarget{{
			ProviderID: "gemini", Model: "flash-lite", Enabled: true,
			RateLimit: config.TargetRateLimit{RequestsPerDay: 1},
		}},
	}
	providerFor := func(string) (provider.Provider, bool) {
		return trackingProvider{id: "gemini", started: providerCalls}, true
	}
	service := NewWithUsageStore([]config.ExecutionProfile{profile}, providerFor, store)
	if _, err := service.Execute(context.Background(), Request{Profile: "free", Prompt: "first"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), Request{Profile: "free", Prompt: "second"}); !errors.Is(err, ErrDailyRateLimit) {
		t.Fatalf("second error = %v, want ErrDailyRateLimit", err)
	}

	restarted := NewWithUsageStore([]config.ExecutionProfile{profile}, providerFor, store)
	if _, err := restarted.Execute(context.Background(), Request{Profile: "free", Prompt: "after restart"}); !errors.Is(err, ErrDailyRateLimit) {
		t.Fatalf("restart error = %v, want persisted ErrDailyRateLimit", err)
	}
	if len(providerCalls) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(providerCalls))
	}
}

func TestTokenLimitRejectsOversizedReservationBeforeProviderCall(t *testing.T) {
	called := make(chan string, 1)
	service := New([]config.ExecutionProfile{{
		ID: "free", Capability: config.CapabilityText, Enabled: true, MaxConcurrency: 1,
		Targets: []config.ExecutionTarget{{
			ProviderID: "gemini", Model: "flash-lite", Enabled: true,
			RateLimit: config.TargetRateLimit{TokensPerMinute: 1000},
		}},
	}}, func(string) (provider.Provider, bool) {
		return trackingProvider{id: "gemini", started: called}, true
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err := service.Execute(ctx, Request{Profile: "free", Prompt: "too large"})
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
	if len(called) != 0 {
		t.Fatal("provider was called before the configured TPM reservation was available")
	}
}
