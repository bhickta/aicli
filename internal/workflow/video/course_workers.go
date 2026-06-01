package video

import (
	"context"
	"errors"
	"fmt"
	"sync"

	progressmodel "github.com/bhickta/aicli/internal/progress"
)

func (s *Service) prepareCourseItems(ctx context.Context, files []string, cacheDir string, slidesDir string, req CourseRequest, skipped []string, progressPlan courseProgressPlan, completedUnits int, progress CourseProgressFunc, totalUnits int) ([]CourseItem, []CourseItem, []string, error) {
	targetNames := courseTargetNames(files)
	workers := normalizedCourseWorkers(courseCompressionWorkers(req), len(files))
	if workers == 1 {
		return s.prepareCourseItemsSequential(ctx, files, targetNames, cacheDir, slidesDir, req, skipped, progressPlan, completedUnits, progress, totalUnits)
	}
	return s.prepareCourseItemsParallel(ctx, files, targetNames, cacheDir, slidesDir, req, skipped, workers, progressPlan, completedUnits, progress, totalUnits)
}

func courseTargetNames(files []string) []string {
	usedNames := map[string]int{}
	targetNames := make([]string, len(files))
	for i, file := range files {
		targetNames[i] = courseTargetName(file, usedNames)
	}
	return targetNames
}

func (s *Service) prepareCourseItemsSequential(ctx context.Context, files []string, targetNames []string, cacheDir string, slidesDir string, req CourseRequest, skipped []string, progressPlan courseProgressPlan, completedUnits int, progress CourseProgressFunc, totalUnits int) ([]CourseItem, []CourseItem, []string, error) {
	items := make([]CourseItem, 0, len(files))
	transcribed := []CourseItem{}
	for i, file := range files {
		item, didTranscribe, err := s.prepareCourseItem(ctx, file, targetNames[i], cacheDir, slidesDir, req)
		if err != nil {
			return nil, nil, nil, err
		}
		if didTranscribe {
			transcribed = append(transcribed, CourseItem{Source: item.Source, SRTPath: item.SRTPath, TextPath: item.TextPath, TargetName: item.TargetName})
			completedUnits += progressPlan.transcriptUnits(file)
		}
		items = append(items, item)
		completedUnits += progressPlan.compressionUnits(file)
		reportCourseProgress(progress, progressmodel.Units(fmt.Sprintf("compressed %d/%d video(s)", i+1, len(files)), completedUnits, totalUnits, "video second"))
	}
	return items, transcribed, skipped, nil
}

func (s *Service) prepareCourseItemsParallel(ctx context.Context, files []string, targetNames []string, cacheDir string, slidesDir string, req CourseRequest, skipped []string, workers int, progressPlan courseProgressPlan, completedUnits int, progress CourseProgressFunc, totalUnits int) ([]CourseItem, []CourseItem, []string, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		index         int
		item          CourseItem
		didTranscribe bool
		err           error
	}
	jobs := make(chan int)
	results := make(chan result, len(files))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}
				item, didTranscribe, err := s.prepareCourseItem(ctx, files[index], targetNames[index], cacheDir, slidesDir, req)
				results <- result{index: index, item: item, didTranscribe: didTranscribe, err: err}
				if err != nil {
					cancel()
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range files {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	items := make([]CourseItem, len(files))
	transcribedByIndex := make([]bool, len(files))
	completed := 0
	var firstErr error
	for res := range results {
		if res.err != nil {
			if firstErr == nil {
				firstErr = res.err
			}
			continue
		}
		items[res.index] = res.item
		transcribedByIndex[res.index] = res.didTranscribe
		completed++
		if res.didTranscribe {
			completedUnits += progressPlan.transcriptUnits(files[res.index])
		}
		completedUnits += progressPlan.compressionUnits(files[res.index])
		reportCourseProgress(progress, progressmodel.Units(fmt.Sprintf("compressed %d/%d video(s) with %d worker(s)", completed, len(files), workers), completedUnits, totalUnits, "video second"))
	}
	if firstErr != nil {
		return nil, nil, nil, firstErr
	}
	if completed != len(files) {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		return nil, nil, nil, errors.New("course processing stopped before all videos completed")
	}

	transcribed := []CourseItem{}
	for i, item := range items {
		if transcribedByIndex[i] {
			transcribed = append(transcribed, CourseItem{Source: item.Source, SRTPath: item.SRTPath, TextPath: item.TextPath, TargetName: item.TargetName})
		}
	}
	return items, transcribed, skipped, nil
}

func normalizedCourseWorkers(workers int, jobs int) int {
	if jobs <= 1 {
		return 1
	}
	if workers <= 0 {
		workers = 2
	}
	if workers > 6 {
		workers = 6
	}
	if workers > jobs {
		return jobs
	}
	return workers
}

func EffectiveCourseWorkers(workers int, jobs int) int {
	return normalizedCourseWorkers(workers, jobs)
}

func courseTranscriptWorkers(req CourseRequest) int {
	if req.TranscriptWorkers > 0 {
		return req.TranscriptWorkers
	}
	return req.Workers
}

func courseCompressionWorkers(req CourseRequest) int {
	if req.CompressionWorkers > 0 {
		return req.CompressionWorkers
	}
	return req.Workers
}

func mergeBatchTranscribedItems(items []CourseItem, transcribed []CourseItem, batchTranscribed map[string]bool) []CourseItem {
	seen := map[string]bool{}
	merged := make([]CourseItem, 0, len(transcribed)+len(batchTranscribed))
	for _, item := range transcribed {
		seen[item.Source] = true
		merged = append(merged, item)
	}
	for _, item := range items {
		if !batchTranscribed[item.Source] || seen[item.Source] {
			continue
		}
		merged = append(merged, CourseItem{Source: item.Source, SRTPath: item.SRTPath, TextPath: item.TextPath, TargetName: item.TargetName})
	}
	return merged
}

func reportCourseProgress(progress CourseProgressFunc, update progressmodel.Update) {
	if progress != nil {
		progress(update)
	}
}
