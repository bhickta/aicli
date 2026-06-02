package video

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	progressmodel "github.com/bhickta/aicli/internal/progress"
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
	courseDir, cacheDir, slidesDir, err := prepareCourseDirs(targetDir, req.OutputDir, req.WorkDir)
	if err != nil {
		return CourseResponse{}, err
	}
	files, durations, skipped, err := s.readableCourseFiles(ctx, files, req.SkipUnreadable)
	if err != nil {
		return CourseResponse{}, err
	}
	if len(files) == 0 {
		return CourseResponse{}, errors.New("no readable video files found")
	}
	resources := systemresources.Collect(ctx)
	req = withCourseWorkerDefaults(req, len(files), resources)
	progressPlan := newCourseProgressPlan(files, durations, cacheDir)
	reportCourseProgress(progress, progressmodel.Units(courseStartStage(len(files), len(skipped)), 0, progressPlan.totalUnits, "video second"))

	if response, pipelined, err := s.runCoursePipeline(ctx, targetDir, courseDir, cacheDir, slidesDir, files, durations, skipped, req, progressPlan, progress); pipelined {
		return s.finishCourse(ctx, req, response, err, progress)
	}

	batchTranscribed, batchAttempted, err := s.prepareMissingTranscriptsWithFasterWhisper(ctx, files, cacheDir, req, progress, progressPlan, progressPlan.totalUnits)
	if err != nil {
		return CourseResponse{}, err
	}
	completedUnits := progressPlan.completedTranscriptUnits(batchTranscribed)
	if batchAttempted && len(batchTranscribed) > 0 {
		reportCourseProgress(progress, progressmodel.Units(
			fmt.Sprintf("transcribed %d/%d video(s) with faster-whisper; compressing", len(batchTranscribed), progressPlan.missingTranscriptCount),
			completedUnits,
			progressPlan.totalUnits,
			"video second",
		))
	}

	items, transcribed, skipped, err := s.prepareCourseItems(ctx, files, cacheDir, slidesDir, req, skipped, progressPlan, completedUnits, progress, progressPlan.totalUnits)
	if err != nil {
		return CourseResponse{}, err
	}
	if len(batchTranscribed) > 0 {
		transcribed = mergeBatchTranscribedItems(items, transcribed, batchTranscribed)
	}
	reportCourseProgress(progress, progressmodel.Units(
		"merging course video, subtitles, and transcript",
		progressPlan.totalUnits-1,
		progressPlan.totalUnits,
		"video second",
	))
	response, err := s.exportCourseParts(ctx, targetDir, courseDir, req.OutputName, items, transcribed, skipped, req.MaxMergeHours)
	return s.finishCourse(ctx, req, response, err, progress)
}
