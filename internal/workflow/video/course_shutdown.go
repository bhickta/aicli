package video

import (
	"context"
	"errors"
	"fmt"
	"strings"

	progressmodel "github.com/bhickta/aicli/internal/progress"
)

func (s *Service) finishCourse(ctx context.Context, req CourseRequest, response CourseResponse, courseErr error, progress CourseProgressFunc) (CourseResponse, error) {
	if courseErr != nil {
		return response, courseErr
	}
	if !req.ShutdownOnComplete {
		return response, nil
	}
	response.ShutdownRequested = true
	reportCourseProgress(progress, progressmodel.Indeterminate("course completed; requesting system shutdown"))
	if err := s.requestSystemShutdown(ctx); err != nil {
		return response, fmt.Errorf("course completed, but shutdown request failed: %w", err)
	}
	return response, nil
}

func (s *Service) requestSystemShutdown(ctx context.Context) error {
	out, err := s.runner.CombinedOutput(ctx, "systemctl", "poweroff")
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(out))
	if message == "" {
		return err
	}
	return errors.New(message + ": " + err.Error())
}
