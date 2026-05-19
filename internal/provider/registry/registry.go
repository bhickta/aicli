package registry

import (
	"net/http"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/provider/ollama"
	"github.com/bhickta/aicli/internal/provider/openai"
)

type Registry struct {
	providers map[string]provider.Provider
}

func New(configs []config.ProviderConfig) *Registry {
	providers := make(map[string]provider.Provider, len(configs))
	client := &http.Client{Timeout: 30 * time.Minute}
	for _, cfg := range configs {
		switch cfg.Type {
		case "ollama":
			providers[cfg.ID] = ollama.New(cfg, client)
		default:
			providers[cfg.ID] = openai.NewCompatible(cfg, client)
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

func (r *Registry) Get(id string) (provider.Provider, bool) {
	p, ok := r.providers[id]
	return p, ok
}
