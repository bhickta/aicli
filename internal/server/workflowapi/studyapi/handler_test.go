package studyapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/storage"
)

func TestRegisterExposesStudyBatchReadRoutes(t *testing.T) {
	t.Parallel()

	db, err := storage.OpenSQLite(filepath.Join(t.TempDir(), "study.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store := storage.NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	batch := storage.StudyBatchRecord{
		ID:     "batch-1",
		Status: "completed",
		Stage:  "all",
		Total:  1,
	}
	if err := store.SaveStudyBatch(context.Background(), batch); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStudyBatchItem(context.Background(), storage.StudyBatchItemRecord{
		BatchID: "batch-1",
		CopyID:  "copy-1",
		Stage:   "all",
		Status:  "ready",
	}); err != nil {
		t.Fatal(err)
	}

	runtime := core.New(core.Dependencies{
		Logger: slog.Default(),
		Store:  store,
	})
	mux := http.NewServeMux()
	New(runtime).Register(mux)

	tests := []struct {
		name       string
		path       string
		assertBody func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "list batches",
			path: "/api/study/batches?limit=10",
			assertBody: func(t *testing.T, res *httptest.ResponseRecorder) {
				t.Helper()
				var payload struct {
					Batches []storage.StudyBatchRecord `json:"batches"`
				}
				if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if len(payload.Batches) != 1 || payload.Batches[0].ID != "batch-1" {
					t.Fatalf("batches = %#v, want batch-1", payload.Batches)
				}
			},
		},
		{
			name: "get batch",
			path: "/api/study/batches/batch-1",
			assertBody: func(t *testing.T, res *httptest.ResponseRecorder) {
				t.Helper()
				var payload struct {
					Batch storage.StudyBatchRecord       `json:"batch"`
					Items []storage.StudyBatchItemRecord `json:"items"`
				}
				if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				if payload.Batch.ID != "batch-1" || len(payload.Items) != 1 {
					t.Fatalf("payload = %#v, want batch-1 with one item", payload)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			res := httptest.NewRecorder()
			mux.ServeHTTP(res, req)
			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", res.Code, res.Body.String())
			}
			tt.assertBody(t, res)
		})
	}
}
