package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

func TestRegistryCreatesVLLMProvider(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"baidu/Unlimited-OCR"}]}`))
	}))
	defer srv.Close()

	registry := New([]config.ProviderConfig{
		{
			ID:      "custom-vllm",
			Type:    "VLLM",
			BaseURL: srv.URL,
		},
	})
	p, ok := registry.Get("custom-vllm")
	if !ok {
		t.Fatal("registry missing custom-vllm provider")
	}
	local, ok := p.(provider.LocalModelServer)
	if !ok {
		t.Fatal("vllm provider does not implement provider.LocalModelServer")
	}
	if !local.LocalModelServer() {
		t.Fatal("LocalModelServer() = false, want true")
	}

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "baidu/Unlimited-OCR" {
		t.Fatalf("models = %#v, want baidu/Unlimited-OCR", models)
	}
}

func TestNormalizeVLLMConfigDefaultsBaseURL(t *testing.T) {
	t.Parallel()

	cfg := normalizeVLLMConfig(config.ProviderConfig{ID: "vllm", Type: "vllm"})
	if cfg.BaseURL != "http://localhost:8000/v1" {
		t.Fatalf("BaseURL = %q, want http://localhost:8000/v1", cfg.BaseURL)
	}
}
