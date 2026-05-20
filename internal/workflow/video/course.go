package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	totalSteps := len(files) + 1
	reportCourseProgress(progress, courseStartStage(len(files), len(skipped)), 0, totalSteps)

	batchTranscribed, batchAttempted, err := s.prepareMissingTranscriptsWithFasterWhisper(ctx, files, cacheDir, req)
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
	reportCourseProgress(progress, "merging course video, subtitles, and transcript", len(files), totalSteps)
	return s.exportCourseParts(ctx, targetDir, courseDir, items, transcribed, skipped, req.MaxMergeHours)
}

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
	return fmt.Sprintf("found %d readable video(s), skipped %d unreadable file(s); preparing transcripts and compressed files", videoCount, skippedCount)
}

func unreadableVideoMessage(file string, err error) string {
	return fmt.Sprintf("Unreadable video %q: %s", filepath.Base(file), err)
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
