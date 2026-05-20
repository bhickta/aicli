package audioapi

import (
	"context"
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
	req, ok := core.DecodeJSON[audio.TranscribeRequest](w, r)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "audio-transcribe", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("transcribing audio"))
		result, err := audio.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Transcribe(ctx, req)
		return result, err
	})
}

func (h *Handler) runAudioAnalyze(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		audio.AnalyzeRequest
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "audio-analyze", "", func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("analyzing audio text"))
		result, err := audio.New(h.runtime.Settings().Tools, tool.ExecRunner{}, p).Analyze(ctx, req.AnalyzeRequest)
		return result, err
	})
}
