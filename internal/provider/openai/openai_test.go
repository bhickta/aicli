package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
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

	p := NewCompatible(config.ProviderConfig{
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

func TestOpenAICompatibleListModelsAddsV1WhenMissing(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		w.Write([]byte(`{"data":[{"id":"model-a"}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "test",
		BaseURL: srv.URL,
	}, srv.Client())

	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "model-a" {
		t.Fatalf("models = %#v, want model-a", models)
	}
}

func TestOpenAICompatibleEmbeddings(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			t.Fatalf("path = %s, want /v1/embeddings", r.URL.Path)
		}
		var body struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Model != "embed" || len(body.Input) != 2 {
			t.Fatalf("body = %#v", body)
		}
		w.Write([]byte(`{"data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}]}`))
	}))
	defer srv.Close()

	p := NewCompatible(config.ProviderConfig{
		ID:      "test",
		BaseURL: srv.URL + "/v1",
	}, srv.Client())

	res, err := p.Embeddings(context.Background(), provider.EmbeddingRequest{
		Model:  "embed",
		Inputs: []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("Embeddings() error = %v", err)
	}
	if len(res.Vectors) != 2 || res.Vectors[0][0] != 1 || res.Vectors[1][1] != 1 {
		t.Fatalf("vectors = %#v", res.Vectors)
	}
}
