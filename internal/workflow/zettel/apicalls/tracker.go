package apicalls

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

var errEmbeddingsUnsupported = errors.New("selected embedding provider does not support embeddings")

type Tracker struct {
	mu        sync.Mutex
	providers map[string]*model.ProviderAPICallUsage
}

type trackedProvider struct {
	provider provider.Provider
	tracker  *Tracker
}

func NewTracker() *Tracker {
	return &Tracker{providers: map[string]*model.ProviderAPICallUsage{}}
}

func (t *Tracker) Wrap(p provider.Provider) provider.Provider {
	if p == nil {
		return nil
	}
	return trackedProvider{provider: p, tracker: t}
}

func (t *Tracker) Snapshot() model.APICallUsage {
	t.mu.Lock()
	defer t.mu.Unlock()

	providers := make([]model.ProviderAPICallUsage, 0, len(t.providers))
	var total model.APICallUsage
	for _, item := range t.providers {
		providerUsage := *item
		providers = append(providers, providerUsage)
		total.Total += providerUsage.Total
		total.Chat += providerUsage.Chat
		total.Embeddings += providerUsage.Embeddings
		total.Vision += providerUsage.Vision
		total.Stream += providerUsage.Stream
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].ProviderID < providers[j].ProviderID
	})
	total.Providers = providers
	return total
}

func (t *Tracker) record(providerID string, mutate func(*model.ProviderAPICallUsage)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	item := t.providers[providerID]
	if item == nil {
		item = &model.ProviderAPICallUsage{ProviderID: providerID}
		t.providers[providerID] = item
	}
	item.Total++
	mutate(item)
}

func (p trackedProvider) ID() string {
	return p.provider.ID()
}

func (p trackedProvider) Health(ctx context.Context) error {
	return p.provider.Health(ctx)
}

func (p trackedProvider) ListModels(ctx context.Context) ([]provider.Model, error) {
	return p.provider.ListModels(ctx)
}

func (p trackedProvider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	p.tracker.record(p.ID(), func(item *model.ProviderAPICallUsage) {
		item.Chat++
	})
	return p.provider.Chat(ctx, req)
}

func (p trackedProvider) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(string) error) error {
	p.tracker.record(p.ID(), func(item *model.ProviderAPICallUsage) {
		item.Stream++
	})
	return p.provider.ChatStream(ctx, req, yield)
}

func (p trackedProvider) Vision(ctx context.Context, req provider.VisionRequest) (provider.ChatResponse, error) {
	p.tracker.record(p.ID(), func(item *model.ProviderAPICallUsage) {
		item.Vision++
	})
	return p.provider.Vision(ctx, req)
}

func (p trackedProvider) Embeddings(ctx context.Context, req provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	embedder, ok := p.provider.(interface {
		Embeddings(context.Context, provider.EmbeddingRequest) (provider.EmbeddingResponse, error)
	})
	if !ok {
		return provider.EmbeddingResponse{}, errEmbeddingsUnsupported
	}
	p.tracker.record(p.ID(), func(item *model.ProviderAPICallUsage) {
		item.Embeddings++
	})
	return embedder.Embeddings(ctx, req)
}
