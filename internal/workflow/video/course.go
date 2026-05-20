package video

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/systemresources"
)

func (s *Service) Course(ctx context.Context, req CourseRequest) (CourseResponse, error) {
	return s.CourseWithProgress(ctx, req, nil)
}

func (s *Service) CourseWithProgress(ctx context.Context, req CourseRequest, progress CourseProgressFunc) (CourseResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return CourseResponse{}, errors.New("path is required")
	}
	source, err := filepath.Abs(req.Path)
	if err != nil {
		return CourseResponse{}, err
	}
	targetDir, err := courseTargetDir(source)
	if err != nil {
		return CourseResponse{}, err
	}
	files, err := rawVideoFiles(source)
	if err != nil {
		return CourseResponse{}, err
	}
	if len(files) == 0 {
		return CourseResponse{}, errors.New("no video files found")
	}
	courseDir, cacheDir, slidesDir, err := prepareCourseDirs(targetDir, req.OutputDir)
	if err != nil {
		return CourseResponse{}, err
	}
	files, skipped, err := s.readableCourseFiles(ctx, files, req.SkipUnreadable)
	if err != nil {
		return CourseResponse{}, err
	}
	if len(files) == 0 {
		return CourseResponse{}, errors.New("no readable video files found")
	}
	resources := systemresources.Collect(ctx)
	req = withCourseWorkerDefaults(req, len(files), resources)
	totalSteps := len(files)*2 + 1
	reportCourseProgress(progress, courseStartStage(len(files), len(skipped)), 0, totalSteps)

	batchTranscribed, batchAttempted, err := s.prepareMissingTranscriptsWithFasterWhisper(ctx, files, cacheDir, req, progress, totalSteps)
	if err != nil {
		return CourseResponse{}, err
	}
	if batchAttempted && len(batchTranscribed) > 0 {
		reportCourseProgress(progress, fmt.Sprintf("transcribed %d/%d video(s) with faster-whisper; compressing", len(batchTranscribed), len(files)), len(batchTranscribed), totalSteps)
	}

	items, transcribed, skipped, err := s.prepareCourseItems(ctx, files, cacheDir, slidesDir, req, skipped, progress, totalSteps)
	if err != nil {
		return CourseResponse{}, err
	}
	if len(batchTranscribed) > 0 {
		transcribed = mergeBatchTranscribedItems(items, transcribed, batchTranscribed)
	}
	reportCourseProgress(progress, "merging course video, subtitles, and transcript", len(files)*2, totalSteps)
	return s.exportCourseParts(ctx, targetDir, courseDir, items, transcribed, skipped, req.MaxMergeHours)
}
