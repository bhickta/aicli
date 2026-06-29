package analyze

import (
	"time"
)

func elapsedMS(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func (s *Service) logInfo(message string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Info(message, args...)
}

func (s *Service) logWarn(message string, args ...any) {
	if s.logger == nil {
		return
	}
	s.logger.Warn(message, args...)
}

func providerID(p interface{ ID() string }) string {
	if p == nil {
		return ""
	}
	return p.ID()
}
