package video

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func chunkCourseItems(ctx context.Context, svc *Service, items []CourseItem, maxMergeHours float64) ([][]CourseItem, error) {
	if len(items) == 0 {
		return nil, errors.New("no compressed videos to merge")
	}
	if maxMergeHours <= 0 {
		return [][]CourseItem{items}, nil
	}
	limit := maxMergeHours * 3600
	chunks := [][]CourseItem{}
	current := []CourseItem{}
	currentSeconds := 0.0
	for _, item := range items {
		duration, err := svc.duration(ctx, item.Source)
		if err != nil {
			duration = 0
		}
		if len(current) > 0 && currentSeconds+duration > limit {
			chunks = append(chunks, current)
			current = nil
			currentSeconds = 0
		}
		current = append(current, item)
		currentSeconds += duration
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks, nil
}

func (s *Service) duration(ctx context.Context, videoPath string) (float64, error) {
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return 0, err
		}
		return 0, errors.New(message + ": " + err.Error())
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func (s *Service) mergeVideos(ctx context.Context, items []CourseItem, outputPath string) error {
	if len(items) == 0 {
		return errors.New("no videos to merge")
	}
	listPath, cleanup, err := writeConcatList(outputPath, items)
	if err != nil {
		return err
	}
	defer cleanup()
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFmpeg, "-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", outputPath)
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return nil
}

func (s *Service) mergeVideosWithSRT(ctx context.Context, items []CourseItem, srtPath string, outputPath string) error {
	if len(items) == 0 {
		return errors.New("no videos to merge")
	}
	listPath, cleanup, err := writeConcatList(outputPath, items)
	if err != nil {
		return err
	}
	defer cleanup()
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFmpeg,
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-i", srtPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-map", "1:s:0",
		"-c", "copy",
		"-c:s", "mov_text",
		outputPath,
	)
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return nil
}

func writeConcatList(outputPath string, items []CourseItem) (string, func(), error) {
	listPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".txt"
	var builder strings.Builder
	for _, item := range items {
		escaped := strings.ReplaceAll(item.Output, "'", "'\\''")
		builder.WriteString("file '")
		builder.WriteString(escaped)
		builder.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(builder.String()), 0o644); err != nil {
		return "", func() {}, err
	}
	return listPath, func() { _ = os.Remove(listPath) }, nil
}

func (s *Service) mergeSRTs(ctx context.Context, items []CourseItem, outputPath string) error {
	var builder strings.Builder
	offset := time.Duration(0)
	index := 1
	for _, item := range items {
		if item.SRTPath != "" && fileExists(item.SRTPath) {
			blocks, err := parseSRT(item.SRTPath)
			if err != nil {
				return err
			}
			for _, block := range blocks {
				builder.WriteString(strconv.Itoa(index))
				builder.WriteString("\n")
				builder.WriteString(formatSRTTime(block.start + offset))
				builder.WriteString(" --> ")
				builder.WriteString(formatSRTTime(block.end + offset))
				builder.WriteString("\n")
				builder.WriteString(block.text)
				builder.WriteString("\n\n")
				index++
			}
		}
		duration, err := s.duration(ctx, item.Output)
		if err != nil {
			duration = 0
		}
		offset += time.Duration(duration * float64(time.Second))
	}
	if builder.Len() == 0 {
		_ = os.Remove(outputPath)
		return nil
	}
	return os.WriteFile(outputPath, []byte(builder.String()), 0o644)
}

func (s *Service) embedSRT(ctx context.Context, videoPath, srtPath, outputPath string) error {
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFmpeg, "-y", "-v", "quiet", "-i", videoPath, "-i", srtPath, "-map", "0:v:0", "-map", "0:a?", "-map", "1:s:0", "-c", "copy", "-c:s", "mov_text", outputPath)
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	_ = os.Remove(videoPath)
	return nil
}

func mergeTranscripts(items []CourseItem, outputPath string) error {
	var builder strings.Builder
	for _, item := range items {
		if item.TextPath == "" || !fileExists(item.TextPath) {
			continue
		}
		data, err := os.ReadFile(item.TextPath)
		if err != nil {
			return err
		}
		stem := strings.TrimSuffix(filepath.Base(item.TextPath), filepath.Ext(item.TextPath))
		builder.WriteString("--- Segment: ")
		builder.WriteString(stem)
		builder.WriteString(" ---\n\n")
		builder.Write(data)
		builder.WriteString("\n\n")
	}
	return os.WriteFile(outputPath, []byte(builder.String()), 0o644)
}
