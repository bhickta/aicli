package ollama

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

func TestOllamaChat(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("path = %s, want /api/chat", r.URL.Path)
		}
		w.Write([]byte(`{"message":{"role":"assistant","content":"hello"}}`))
	}))
	defer srv.Close()

	p := New(config.ProviderConfig{
		ID:      "ollama",
		BaseURL: srv.URL,
		Model:   "llama",
	}, srv.Client())

	res, err := p.Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if res.Content != "hello" {
		t.Fatalf("Content = %q, want hello", res.Content)
	}
}
