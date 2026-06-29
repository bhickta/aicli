package registry

import (
	"net/http"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/provider/codexcli"
	"github.com/bhickta/aicli/internal/provider/geminicli"
	"github.com/bhickta/aicli/internal/provider/ollama"
	"github.com/bhickta/aicli/internal/provider/openai"
	"github.com/bhickta/aicli/internal/tool"
)

type Registry struct {
	providers map[string]provider.Provider
}

func New(configs []config.ProviderConfig, toolConfigs ...config.ToolConfig) *Registry {
	providers := make(map[string]provider.Provider, len(configs))
	client := &http.Client{Timeout: 30 * time.Minute}
	tools := config.DefaultSettings().Tools
	if len(toolConfigs) > 0 {
		tools = config.Normalize(config.Settings{Tools: toolConfigs[0]}).Tools
	}
	for _, cfg := range configs {
		switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
		case "ollama":
			providers[cfg.ID] = ollama.New(cfg, client)
		case "vllm":
			providers[cfg.ID] = openai.NewCompatible(normalizeVLLMConfig(cfg), client)
		case "codex-cli":
			providers[cfg.ID] = codexcli.New(cfg, tools, tool.ExecRunner{})
		case "gemini-cli":
			providers[cfg.ID] = geminicli.New(cfg, tools, tool.ExecRunner{})
		default:
			providers[cfg.ID] = openai.NewCompatible(cfg, client)
		}
	}
	return &Registry{providers: providers}
}

func normalizeVLLMConfig(cfg config.ProviderConfig) config.ProviderConfig {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:8000/v1"
	}
	return cfg
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
