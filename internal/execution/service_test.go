package execution

import (
	"context"
	"errors"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

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
