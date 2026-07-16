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
	writeJSON(w, http.StatusOK, map[string]any{"providers": sanitizedProviders(s.settingsSnapshot().Providers)})
}

func (s *Server) providerModels(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/")
	if len(parts) != 2 || parts[1] != "models" {
		http.NotFound(w, r)
		return
	}
	p, ok := s.providerFor(parts[0])
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	models, err := p.ListModels(r.Context())
	if err != nil {
		if s.deps.Logger != nil {
			s.deps.Logger.Warn("provider model list failed", "provider", parts[0], "error", err)
		}
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
	tools := s.settingsSnapshot().Tools
	statuses := []tool.Status{
		checker.Check(r.Context(), "ffmpeg", tools.FFmpeg, "-version"),
		checker.Check(r.Context(), "ffprobe", tools.FFprobe, "-version"),
		checker.Check(r.Context(), "pdftoppm", tools.PDFToPPM, "-v"),
		checker.Check(r.Context(), "whisper", tools.WhisperCLI, "--help"),
		checker.Check(r.Context(), "codex", tools.CodexCLI, "--version"),
		checker.Check(r.Context(), "ots.TTS", tools.OTSTTS, "--help"),
		checker.Check(r.Context(), "firefox", tools.Firefox, "--version"),
		checker.Check(r.Context(), "xdotool", tools.XDoTool, "--version"),
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": statuses})
}

func (s *Server) providerFor(id string) (provider.Provider, bool) {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	if id == "" {
		id = s.deps.Settings.DefaultProvider
	}
	return s.deps.Providers.Get(id)
}
