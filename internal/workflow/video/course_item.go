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

func (s *Service) prepareCourseItem(
	ctx context.Context,
	file string,
	targetName string,
	cacheDir string,
	slidesDir string,
	req CourseRequest,
) (CourseItem, bool, error) {
	srtPath, textPath, didTranscribe, err := s.prepareTranscriptFiles(
		ctx,
		file,
		cacheDir,
		req.WhisperModel,
		req.WhisperDevice,
	)
	if err != nil {
		return CourseItem{}, false, err
	}

	output := filepath.Join(slidesDir, targetName+"_slideshow.mp4")
	item := CourseItem{
		Source:     file,
		Output:     output,
		SRTPath:    srtPath,
		TextPath:   textPath,
		TargetName: targetName,
	}
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
		Path:          file,
		Output:        output,
		Resolution:    req.Resolution,
		Preset:        req.Preset,
		CRF:           req.CRF,
		FPS:           req.FPS,
		FastSkip:      true,
		ExternalSRT:   srtPath,
		SkipSubtitles: true,
		SkipFastStart: true,
		TargetName:    targetName,
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
