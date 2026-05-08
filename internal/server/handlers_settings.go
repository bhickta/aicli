package server

import (
	"encoding/json"
	"net/http"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Settings)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var settings config.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Save(s.deps.SettingsPath, settings); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.deps.Settings = settings
	s.deps.Providers = provider.NewRegistry(settings.Providers)
	writeJSON(w, http.StatusOK, settings)
}
