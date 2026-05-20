package video

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
)

func (s *Service) readableCourseFiles(ctx context.Context, files []string, skipUnreadable bool) ([]string, []string, error) {
	readable := make([]string, 0, len(files))
	skipped := []string{}
	for _, file := range files {
		if _, err := s.duration(ctx, file); err != nil {
			message := unreadableVideoMessage(file, err)
			if !skipUnreadable {
				return nil, nil, errors.New(message)
			}
			skipped = append(skipped, message)
			continue
		}
		readable = append(readable, file)
	}
	return readable, skipped, nil
}

func courseStartStage(videoCount, skippedCount int) string {
	if skippedCount == 0 {
		return fmt.Sprintf("found %d video(s); preparing transcripts and compressed files", videoCount)
	}
	return fmt.Sprintf(
		"found %d readable video(s), skipped %d unreadable file(s); preparing transcripts and compressed files",
		videoCount,
		skippedCount,
	)
}

func unreadableVideoMessage(file string, err error) string {
	return fmt.Sprintf("Unreadable video %q: %s", filepath.Base(file), err)
}
