package server

import (
	"encoding/json"
	"net/http"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider/registry"
)

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, sanitizedSettings(s.settingsSnapshot()))
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var settings config.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	settings = config.Normalize(settings)
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	settings.Providers = preserveProviderSecrets(settings.Providers, s.deps.Settings.Providers)
	if err := config.Save(s.deps.SettingsPath, settings); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.deps.Settings = settings
	s.deps.Providers = registry.New(settings.Providers, settings.Tools)
	s.execution.UpdateProfiles(settings.ExecutionProfiles)
	writeJSON(w, http.StatusOK, sanitizedSettings(settings))
}
