package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
)

func (s *Server) listProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.deps.Settings.Providers})
}

func (s *Server) providerModels(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/")
	if len(parts) != 2 || parts[1] != "models" {
		http.NotFound(w, r)
		return
	}
	p, ok := s.deps.Providers.Get(parts[0])
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	models, err := p.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		provider.ChatRequest
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
	res, err := p.Chat(r.Context(), req.ChatRequest)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) chatStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		provider.ChatRequest
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
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	err := p.ChatStream(r.Context(), req.ChatRequest, func(chunk string) error {
		_, writeErr := fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(chunk, "\n", "\\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return writeErr
	})
	if err != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", strings.ReplaceAll(err.Error(), "\n", " "))
		return
	}
	_, _ = fmt.Fprint(w, "event: done\ndata: {}\n\n")
}

func (s *Server) tools(w http.ResponseWriter, r *http.Request) {
	checker := tool.Checker{}
	statuses := []tool.Status{
		checker.Check(r.Context(), "ffmpeg", s.deps.Settings.Tools.FFmpeg, "-version"),
		checker.Check(r.Context(), "ffprobe", s.deps.Settings.Tools.FFprobe, "-version"),
		checker.Check(r.Context(), "pdftoppm", s.deps.Settings.Tools.PDFToPPM, "-v"),
		checker.Check(r.Context(), "whisper-cli", s.deps.Settings.Tools.WhisperCLI, "--help"),
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": statuses})
}

func (s *Server) providerFor(id string) (provider.Provider, bool) {
	if id == "" {
		id = s.deps.Settings.DefaultProvider
	}
	return s.deps.Providers.Get(id)
}
