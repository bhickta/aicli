package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/audio"
)

func (s *Server) runAudioTranscribe(w http.ResponseWriter, r *http.Request) {
	var req audio.TranscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("audio-transcribe", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("transcribing audio", 2, 4)
		result, err := audio.New(s.deps.Settings.Tools, tool.ExecRunner{}).Transcribe(ctx, req)
		progress("saving transcript", 3, 4)
		return result, err
	})
}

func (s *Server) runAudioAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		audio.AnalyzeRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("audio-analyze", "")
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("analyzing audio text", 2, 4)
		result, err := audio.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Analyze(ctx, req.AnalyzeRequest)
		progress("saving analysis", 3, 4)
		return result, err
	})
}
