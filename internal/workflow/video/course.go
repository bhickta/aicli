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
	items := make([]CourseItem, 0, len(files))
	transcribed := []CourseItem{}
	skipped := []string{}
	usedNames := map[string]int{}
	for _, file := range files {
		item, didTranscribe, err := s.prepareCourseItem(ctx, file, cacheDir, slidesDir, req, usedNames)
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

func (s *Service) prepareCourseItem(ctx context.Context, file string, cacheDir string, slidesDir string, req CourseRequest, usedNames map[string]int) (CourseItem, bool, error) {
	targetName := courseTargetName(file, usedNames)
	srtPath, textPath, didTranscribe, err := s.prepareTranscriptFiles(ctx, file, cacheDir, req.WhisperModel)
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

func (s *Service) exportCourseParts(ctx context.Context, targetDir string, courseDir string, items []CourseItem, transcribed []CourseItem, skipped []string, maxMergeHours float64) (CourseResponse, error) {
	parts, err := chunkCourseItems(ctx, s, items, maxMergeHours)
	if err != nil {
		return CourseResponse{}, err
	}
	folderName := filepath.Base(targetDir)
	response := CourseResponse{CourseDir: courseDir, Compressed: items, Transcribed: transcribed, Skipped: skipped}
	multipart := len(parts) > 1
	for i, part := range parts {
		artifact := courseArtifactPaths(courseDir, folderName, multipart, i)
		if err := s.writeCoursePart(ctx, part, artifact); err != nil {
			return CourseResponse{}, err
		}
		if i == 0 {
			response.VideoPath = artifact.videoPath
			response.SRTPath = artifact.srtPath
			response.TextPath = artifact.textPath
		}
	}
	return response, nil
}

type courseArtifact struct {
	videoPath    string
	tmpVideoPath string
	srtPath      string
	textPath     string
}

func courseArtifactPaths(courseDir string, folderName string, multipart bool, index int) courseArtifact {
	suffix := ""
	if multipart {
		suffix = fmt.Sprintf("_Part%d", index+1)
	}
	return courseArtifact{
		videoPath:    filepath.Join(courseDir, folderName+suffix+"_Slideshow.mp4"),
		tmpVideoPath: filepath.Join(courseDir, folderName+suffix+"_tmp.mp4"),
		srtPath:      filepath.Join(courseDir, folderName+suffix+".srt"),
		textPath:     filepath.Join(courseDir, folderName+suffix+".txt"),
	}
}

func (s *Service) writeCoursePart(ctx context.Context, part []CourseItem, artifact courseArtifact) error {
	if err := s.mergeSRTs(ctx, part, artifact.srtPath); err != nil {
		return err
	}
	if err := s.mergeVideos(ctx, part, artifact.tmpVideoPath); err != nil {
		return err
	}
	if _, err := os.Stat(artifact.srtPath); err == nil {
		if err := s.embedSRT(ctx, artifact.tmpVideoPath, artifact.srtPath, artifact.videoPath); err != nil {
			return err
		}
	} else if err := os.Rename(artifact.tmpVideoPath, artifact.videoPath); err != nil {
		return err
	}
	return mergeTranscripts(part, artifact.textPath)
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
