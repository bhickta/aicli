package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

func (s *Service) Course(ctx context.Context, req CourseRequest) (CourseResponse, error) {
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

	items, transcribed, skipped, err := s.prepareCourseItems(ctx, files, cacheDir, slidesDir, req)
	if err != nil {
		return CourseResponse{}, err
	}
	return s.exportCourseParts(ctx, targetDir, courseDir, items, transcribed, skipped, req.MaxMergeHours)
}

func courseTargetDir(source string) (string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return source, nil
	}
	return filepath.Dir(source), nil
}

func prepareCourseDirs(targetDir string, outputDir string) (string, string, string, error) {
	courseDir := outputDir
	if strings.TrimSpace(courseDir) == "" {
		courseDir = filepath.Join(targetDir, "Course")
	}
	if err := os.MkdirAll(courseDir, 0o755); err != nil {
		return "", "", "", err
	}
	cacheDir := filepath.Join(targetDir, ".aicli_cache")
	slidesDir := filepath.Join(cacheDir, "slideshows")
	if err := os.MkdirAll(slidesDir, 0o755); err != nil {
		return "", "", "", err
	}
	return courseDir, cacheDir, slidesDir, nil
}

func (s *Service) prepareCourseItems(ctx context.Context, files []string, cacheDir string, slidesDir string, req CourseRequest) ([]CourseItem, []CourseItem, []string, error) {
	skipped := []string{}
	usedNames := map[string]int{}
	targetNames := make([]string, len(files))
	for i, file := range files {
		targetNames[i] = courseTargetName(file, usedNames)
	}

	workers := normalizedCourseWorkers(req.Workers, len(files))
	if workers == 1 {
		return s.prepareCourseItemsSequential(ctx, files, targetNames, cacheDir, slidesDir, req, skipped)
	}
	return s.prepareCourseItemsParallel(ctx, files, targetNames, cacheDir, slidesDir, req, skipped, workers)
}

func (s *Service) prepareCourseItemsSequential(ctx context.Context, files []string, targetNames []string, cacheDir string, slidesDir string, req CourseRequest, skipped []string) ([]CourseItem, []CourseItem, []string, error) {
	items := make([]CourseItem, 0, len(files))
	transcribed := []CourseItem{}
	for i, file := range files {
		item, didTranscribe, err := s.prepareCourseItem(ctx, file, targetNames[i], cacheDir, slidesDir, req)
		if err != nil {
			return nil, nil, nil, err
		}
		if didTranscribe {
			transcribed = append(transcribed, CourseItem{Source: item.Source, SRTPath: item.SRTPath, TextPath: item.TextPath, TargetName: item.TargetName})
		}
		items = append(items, item)
	}
	return items, transcribed, skipped, nil
}

func (s *Service) prepareCourseItemsParallel(ctx context.Context, files []string, targetNames []string, cacheDir string, slidesDir string, req CourseRequest, skipped []string, workers int) ([]CourseItem, []CourseItem, []string, error) {
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

func (s *Service) prepareCourseItem(ctx context.Context, file string, targetName string, cacheDir string, slidesDir string, req CourseRequest) (CourseItem, bool, error) {
	srtPath, textPath, didTranscribe, err := s.prepareTranscriptFiles(ctx, file, cacheDir, req.WhisperModel, req.WhisperDevice)
	if err != nil {
		return CourseItem{}, false, err
	}
	output := filepath.Join(slidesDir, targetName+"_slideshow.mp4")
	item := CourseItem{Source: file, Output: output, SRTPath: srtPath, TextPath: textPath, TargetName: targetName}
	if _, statErr := os.Stat(output); statErr == nil {
		return item, didTranscribe, nil
	}
	compressReq := courseCompressRequest(file, output, srtPath, targetName, req)
	output, out, compressErr := s.compress(ctx, compressReq)
	if compressErr != nil {
		return CourseItem{}, false, errors.New(strings.TrimSpace(string(out)) + ": " + compressErr.Error())
	}
	item.Output = output
	return item, didTranscribe, nil
}

func courseCompressRequest(file string, output string, srtPath string, targetName string, req CourseRequest) CompressRequest {
	compressReq := CompressRequest{
		Path:        file,
		Output:      output,
		Resolution:  req.Resolution,
		Preset:      req.Preset,
		CRF:         req.CRF,
		FPS:         req.FPS,
		FastSkip:    true,
		ExternalSRT: srtPath,
		TargetName:  targetName,
	}
	if req.FastSkip {
		compressReq.FastSkip = true
	}
	if compressReq.Preset == "" {
		compressReq.Preset = "slideshow"
	}
	if compressReq.FPS == "" {
		compressReq.FPS = "1/2"
	}
	if compressReq.Resolution == 0 {
		compressReq.Resolution = -1
	}
	return compressReq
}

func courseTargetName(videoPath string, used map[string]int) string {
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	sanitized := sanitizeCourseName(stem)
	if sanitized == "" {
		sanitized = "video"
	}
	suffix := stem
	if len(suffix) > 15 {
		suffix = suffix[len(suffix)-15:]
	}
	target := sanitizeCourseName(sanitized + "_" + suffix)
	if count := used[target]; count > 0 {
		used[target] = count + 1
		return fmt.Sprintf("%s_%d", target, count+1)
	}
	used[target] = 1
	return target
}

func sanitizeCourseName(value string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9 ._-]+`)
	return strings.TrimSpace(re.ReplaceAllString(value, ""))
}
