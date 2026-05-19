package audioapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/audio"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/audio/transcribe", h.runAudioTranscribe)
	mux.HandleFunc("POST /api/workflows/audio/analyze", h.runAudioAnalyze)
}

func (h *Handler) runAudioTranscribe(w http.ResponseWriter, r *http.Request) {
	var req audio.TranscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	job := core.NewJob("audio-transcribe", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("transcribing audio", 2, 4)
		result, err := audio.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Transcribe(ctx, req)
		progress("saving transcript", 3, 4)
		return result, err
	})
}

func (h *Handler) runAudioAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		audio.AnalyzeRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("audio-analyze", "")
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("analyzing audio text", 2, 4)
		result, err := audio.New(h.runtime.Settings().Tools, tool.ExecRunner{}, p).Analyze(ctx, req.AnalyzeRequest)
		progress("saving analysis", 3, 4)
		return result, err
	})
}
