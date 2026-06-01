package zettelapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/storage"
	zettel "github.com/bhickta/aicli/internal/workflow/zettel/api"
)

type zettelAPITestProvider struct {
	id        string
	embedding bool
}

func (p zettelAPITestProvider) ID() string {
	return p.id
}

func (p zettelAPITestProvider) Health(context.Context) error {
	return nil
}

func (p zettelAPITestProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (p zettelAPITestProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("chat should not be called")
}

func (p zettelAPITestProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return errors.New("chat stream should not be called")
}

func (p zettelAPITestProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("vision should not be called")
}

func (p zettelAPITestProvider) Embeddings(context.Context, provider.EmbeddingRequest) (provider.EmbeddingResponse, error) {
	if !p.embedding {
		return provider.EmbeddingResponse{}, errors.New("embeddings should not be called")
	}
	return provider.EmbeddingResponse{Vectors: [][]float64{{1, 1}}}, nil
}

func TestServiceForIndexUsesResolvedEmbeddingProviderBeforeFallback(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeZettelAPITestFile(t, filepath.Join(vaultDir, "zettelkasten", "note.md"), "# Note\n")
	providers := map[string]provider.Provider{
		"embed": zettelAPITestProvider{id: "embed", embedding: true},
		"chat":  noEmbeddingZettelAPITestProvider{id: "chat"},
	}
	handler := New(core.New(core.Dependencies{
		Logger:  slog.Default(),
		DataDir: t.TempDir(),
		Settings: func() config.Settings {
			return config.Settings{DefaultProvider: "chat"}
		},
		ProviderFor: func(id string) (provider.Provider, bool) {
			p, ok := providers[id]
			return p, ok
		},
	}))
	service, err := handler.serviceFor(zettel.Options{
		VaultPath:  vaultDir,
		ProviderID: "embed",
	}, providerNeeds{embedding: true})
	if err != nil {
		t.Fatalf("serviceFor() error = %v", err)
	}

	resp, err := service.Index(context.Background(), zettel.IndexRequest{
		Options: zettel.Options{
			VaultPath:  vaultDir,
			ProviderID: "embed",
		},
	}, nil)
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if resp.Updated != 1 {
		t.Fatalf("Index() = %#v, want one embedded note", resp)
	}
}

func TestTrainingExportRouteStartsLocalJob(t *testing.T) {
	t.Parallel()

	db, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := storage.NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	vaultDir := t.TempDir()
	runtime := core.New(core.Dependencies{
		Logger:  slog.Default(),
		Store:   store,
		DataDir: t.TempDir(),
		Settings: func() config.Settings {
			return config.DefaultSettings()
		},
		ProviderFor: func(string) (provider.Provider, bool) {
			return nil, false
		},
	})
	mux := http.NewServeMux()
	New(runtime).Register(mux)

	body := `{"vault_path":` + jsonQuote(vaultDir) + `,"data_folder":".aicli-zettel-merge"}`
	req := httptest.NewRequest(http.MethodPost, "/api/workflows/zettel/training-export", strings.NewReader(body))
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202, body=%s", res.Code, res.Body.String())
	}

	var payload struct {
		Job storage.Job `json:"job"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Job.Type != "zettel-training-export" {
		t.Fatalf("job type = %q, want zettel-training-export", payload.Job.Type)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		job, err := store.GetJob(context.Background(), payload.Job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if job.Status == storage.JobStatusCompleted {
			return
		}
		if job.Status == storage.JobStatusFailed {
			t.Fatalf("training export job failed: %s", job.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("training export job did not complete")
}

func jsonQuote(value string) string {
	data, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(data)
}

type noEmbeddingZettelAPITestProvider struct {
	id string
}

func (p noEmbeddingZettelAPITestProvider) ID() string {
	return p.id
}

func (p noEmbeddingZettelAPITestProvider) Health(context.Context) error {
	return nil
}

func (p noEmbeddingZettelAPITestProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}

func (p noEmbeddingZettelAPITestProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("chat should not be called")
}

func (p noEmbeddingZettelAPITestProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return errors.New("chat stream should not be called")
}

func (p noEmbeddingZettelAPITestProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("vision should not be called")
}

func writeZettelAPITestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
