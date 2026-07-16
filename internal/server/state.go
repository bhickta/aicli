package server

import "github.com/bhickta/aicli/internal/config"

func (s *Server) settingsSnapshot() config.Settings {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.deps.Settings
}
