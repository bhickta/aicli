package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/execution"
	"github.com/bhickta/aicli/internal/provider/registry"
)

func (s *Server) executionControl(w http.ResponseWriter, _ *http.Request) {
	settings := s.settingsSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{
		"providers":    sanitizedProviders(settings.Providers),
		"profiles":     settings.ExecutionProfiles,
		"capabilities": []string{"text", "structured", "vision", "embedding", "rerank", "ocr"},
	})
}

func (s *Server) updateExecutionProfiles(w http.ResponseWriter, r *http.Request) {
	var profiles []config.ExecutionProfile
	if err := json.NewDecoder(r.Body).Decode(&profiles); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	profiles = config.NormalizeExecutionProfiles(profiles)
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if err := execution.ValidateProfiles(profiles, configuredProviderIDs(s.deps.Settings.Providers)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	settings := s.deps.Settings
	settings.ExecutionProfiles = profiles
	if err := config.Save(s.deps.SettingsPath, settings); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.deps.Settings = settings
	s.execution.UpdateProfiles(profiles)
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) updateExecutionProviders(w http.ResponseWriter, r *http.Request) {
	var providers []config.ProviderConfig
	if err := json.NewDecoder(r.Body).Decode(&providers); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	providers = preserveProviderSecrets(providers, s.deps.Settings.Providers)
	if err := validateProviders(providers); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	settings := s.deps.Settings
	settings.Providers = providers
	if err := config.Save(s.deps.SettingsPath, settings); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.deps.Settings = config.Normalize(settings)
	s.deps.Providers = registry.New(s.deps.Settings.Providers, s.deps.Settings.Tools)
	writeJSON(w, http.StatusOK, sanitizedProviders(s.deps.Settings.Providers))
}

func (s *Server) executionModels(w http.ResponseWriter, r *http.Request) {
	providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
	provider, ok := s.providerFor(providerID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	models, err := provider.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (s *Server) executionHealth(w http.ResponseWriter, r *http.Request) {
	providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
	provider, ok := s.providerFor(providerID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	if err := provider.Health(r.Context()); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "provider_id": providerID})
}
