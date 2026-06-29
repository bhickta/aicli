package studyapi

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/lecture"
	"github.com/bhickta/aicli/internal/workflow/recall"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/study/copies", h.listStudyCopies)
	mux.HandleFunc("POST /api/study/imports", h.importStudyCopies)
	mux.HandleFunc("GET /api/study/copies/{id}", h.getStudyCopy)
	mux.HandleFunc("PUT /api/study/copies/{id}", h.updateStudyCopy)
	mux.HandleFunc("POST /api/study/copies/{id}/stages", h.startStudyStage)
	mux.HandleFunc("POST /api/study/batches", h.startStudyBatch)
	mux.HandleFunc("POST /api/workflows/recall/run", h.runRecall)
	mux.HandleFunc("POST /api/workflows/study/lecture", h.runLecture)
}

func (h *Handler) runRecall(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		Notes      string `json:"notes"`
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}

	h.runtime.StartJob(w, r, "recall", req.Notes, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("generating recall triggers"))
		result, err := recall.New(p).Generate(ctx, recall.Request{Model: req.Model, Notes: req.Notes})
		return result, err
	})
}

func (h *Handler) runLecture(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		lecture.Request
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	job := core.NewJob("study-lecture", req.SourcePath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		artifactDir := ""
		if h.runtime.DataDir() != "" {
			artifactDir = filepath.Join(h.runtime.DataDir(), "artifacts", "lectures")
		}
		result, err := lecture.New(
			p,
			h.runtime.Settings().Tools,
			tool.ExecRunner{},
			lecture.WithArtifactDir(artifactDir),
		).Run(ctx, req.Request, func(stage string, completed int, total int, label string) {
			progress(core.Units(stage, completed, total, label))
		})
		return result, err
	})
}
