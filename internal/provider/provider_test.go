package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

func TestOpenAICompatibleListModels(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
	}))
	defer srv.Close()

	p := NewOpenAICompatible(config.ProviderConfig{
		ID:      "test",
		BaseURL: srv.URL + "/v1",
		APIKey:  "key",
	}, srv.Client())

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-a" {
		t.Fatalf("models = %#v, want model-a", models)
	}
}

func TestOllamaChat(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %s, want /api/chat", r.URL.Path)
		}
		w.Write([]byte(`{"message":{"role":"assistant","content":"hello"}}`))
	}))
	defer srv.Close()

	p := NewOllama(config.ProviderConfig{
		ID:      "ollama",
		BaseURL: srv.URL,
		Model:   "llama",
	}, srv.Client())

	res, err := p.Chat(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if res.Content != "hello" {
		t.Fatalf("Content = %q, want hello", res.Content)
	}
}
