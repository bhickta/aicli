package provider

import (
	"context"
	"net/http"
	"time"

	"github.com/bhickta/aicli/internal/config"
)

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type ChatResponse struct {
	Content string `json:"content"`
}

type VisionRequest struct {
	Model       string  `json:"model"`
	Prompt      string  `json:"prompt"`
	Image       []byte  `json:"-"`
	MIMEType    string  `json:"mime_type"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"max_tokens"`
}

type Provider interface {
	ID() string
	Health(ctx context.Context) error
	ListModels(ctx context.Context) ([]Model, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest, yield func(string) error) error
	Vision(ctx context.Context, req VisionRequest) (ChatResponse, error)
}

type Registry struct {
	providers map[string]Provider
}

func NewRegistry(configs []config.ProviderConfig) *Registry {
	providers := make(map[string]Provider, len(configs))
	client := &http.Client{Timeout: 30 * time.Minute}
	for _, cfg := range configs {
		switch cfg.Type {
		case "ollama":
			providers[cfg.ID] = NewOllama(cfg, client)
		default:
			providers[cfg.ID] = NewOpenAICompatible(cfg, client)
		}
	}
	return &Registry{providers: providers}
}

func (r *Registry) List() []string {
	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

func (r *Registry) Get(id string) (Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}
