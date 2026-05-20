package video

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	"github.com/bhickta/aicli/internal/workflow/whisper"
)

func (s *Service) prepareTranscriptFiles(ctx context.Context, videoPath, cacheDir, whisperModel string, whisperDevice string) (string, string, bool, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", false, err
	}
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	cacheSRT := filepath.Join(cacheDir, stem+".srt")
	cacheText := filepath.Join(cacheDir, stem+".txt")
	sidecarSRT := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".srt"
	didTranscribe := false
	if !fileExists(cacheSRT) && fileExists(sidecarSRT) {
		if err := copyFile(sidecarSRT, cacheSRT); err != nil {
			return "", "", false, err
		}
	}
	if !fileExists(cacheSRT) {
		if err := s.transcribeVideo(ctx, videoPath, strings.TrimSuffix(cacheSRT, filepath.Ext(cacheSRT)), whisperModel, whisperDevice); err != nil {
			return "", "", false, err
		}
		didTranscribe = true
	}
	if fileExists(cacheSRT) {
		text, err := srtToText(cacheSRT)
		if err != nil {
			return cacheSRT, "", didTranscribe, err
		}
		if err := os.WriteFile(cacheText, []byte(text), 0o644); err != nil {
			return cacheSRT, "", didTranscribe, err
		}
	}
	if !fileExists(cacheSRT) {
		cacheSRT = ""
	}
	if !fileExists(cacheText) {
		cacheText = ""
	}
	if cacheSRT == "" || cacheText == "" {
		return cacheSRT, cacheText, didTranscribe, errors.New("transcription did not produce both .srt and .txt")
	}
	return cacheSRT, cacheText, didTranscribe, nil
}

func (s *Service) prepareMissingTranscriptsWithFasterWhisper(ctx context.Context, files []string, cacheDir string, req CourseRequest, progress CourseProgressFunc, progressPlan courseProgressPlan, totalUnits int) (map[string]bool, bool, error) {
	transcribed := map[string]bool{}
	if !whisper.CanRunFasterBatch(s.runner) {
		return transcribed, false, nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return transcribed, false, err
	}

	missing := []string{}
	for _, file := range files {
		cacheSRT, cacheText, sidecarSRT := transcriptPaths(file, cacheDir)
		if !fileExists(cacheSRT) && fileExists(sidecarSRT) {
			if err := copyFile(sidecarSRT, cacheSRT); err != nil {
				return transcribed, false, err
			}
		}
		if fileExists(cacheSRT) {
			if !fileExists(cacheText) {
				if err := writeTranscriptText(cacheSRT, cacheText); err != nil {
					return transcribed, false, err
				}
			}
			continue
		}
		missing = append(missing, file)
	}
	if len(missing) == 0 {
		return transcribed, true, nil
	}
	if strings.TrimSpace(s.tools.WhisperCLI) == "" {
		return transcribed, false, errors.New("whisper is not configured")
	}

	model := req.WhisperModel
	if model == "" {
		model = "large-v3"
	}
	var completedFiles atomic.Int64
	var completedUnits atomic.Int64
	missingByBase := make(map[string]string, len(missing))
	for _, file := range missing {
		missingByBase[filepath.Base(file)] = file
	}
	out, err := whisper.RunFasterBatchStreaming(ctx, s.runner, whisper.FasterBatchRequest{
		AudioPaths: missing,
		OutputDir:  cacheDir,
		Model:      model,
		Device:     req.WhisperDevice,
		Workers:    normalizedCourseWorkers(courseTranscriptWorkers(req), len(missing)),
		BatchSize:  24,
		BeamSize:   1,
	}, func(line string) {
		file := transcribedLinePath(line, missingByBase)
		if file == "" {
			return
		}
		currentFile := int(completedFiles.Add(1))
		currentUnits := int(completedUnits.Add(int64(progressPlan.transcriptUnits(file))))
		reportCourseProgress(progress, progressmodel.Units(
			fmt.Sprintf("transcribed %d/%d video(s): %s", currentFile, len(missing), filepath.Base(file)),
			currentUnits,
			totalUnits,
			"video second",
		))
	})
	if err != nil {
		if whisper.FasterBatchUnavailable(out, err) {
			return transcribed, false, nil
		}
		return transcribed, true, whisper.OutputError(out, err)
	}
	for _, file := range missing {
		cacheSRT, cacheText, _ := transcriptPaths(file, cacheDir)
		if !fileExists(cacheText) && fileExists(cacheSRT) {
			if err := writeTranscriptText(cacheSRT, cacheText); err != nil {
				return transcribed, true, err
			}
		}
		if !fileExists(cacheSRT) || !fileExists(cacheText) {
			return transcribed, true, errors.New("faster-whisper did not produce both .srt and .txt for " + filepath.Base(file))
		}
		transcribed[file] = true
	}
	return transcribed, true, nil
}

func transcribedLinePath(line string, byBase map[string]string) string {
	const prefix = "transcribed "
	if !strings.HasPrefix(line, prefix) {
		return ""
	}
	raw := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if raw == "" {
		return ""
	}
	if file, ok := byBase[filepath.Base(raw)]; ok {
		return file
	}
	return raw
}

func transcriptPaths(videoPath, cacheDir string) (string, string, string) {
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	cacheSRT := filepath.Join(cacheDir, stem+".srt")
	cacheText := filepath.Join(cacheDir, stem+".txt")
	sidecarSRT := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".srt"
	return cacheSRT, cacheText, sidecarSRT
}

func writeTranscriptText(srtPath, textPath string) error {
	text, err := srtToText(srtPath)
	if err != nil {
		return err
	}
	return os.WriteFile(textPath, []byte(text), 0o644)
}

func (s *Service) transcribeVideo(ctx context.Context, videoPath, outputBase, whisperModel string, whisperDevice string) error {
	if strings.TrimSpace(s.tools.WhisperCLI) == "" {
		return errors.New("whisper is not configured")
	}
	if whisperModel == "" {
		whisperModel = "large-v3"
	}
	out, err := whisper.Run(ctx, s.runner, whisper.Request{
		Command:    s.tools.WhisperCLI,
		AudioPath:  videoPath,
		OutputBase: outputBase,
		Model:      whisperModel,
		Device:     whisperDevice,
		SRT:        true,
		Text:       true,
	})
	if err != nil {
		return whisper.OutputError(out, err)
	}
	return nil
}

type srtBlock struct {
	start time.Duration
	end   time.Duration
	text  string
}

func parseSRT(path string) ([]srtBlock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	chunks := regexp.MustCompile(`\r?\n\r?\n`).Split(strings.TrimSpace(string(data)), -1)
	blocks := []srtBlock{}
	for _, chunk := range chunks {
		lines := strings.Split(strings.ReplaceAll(chunk, "\r\n", "\n"), "\n")
		if len(lines) < 3 || !strings.Contains(lines[1], " --> ") {
			continue
		}
		parts := strings.SplitN(lines[1], " --> ", 2)
		start, err := parseSRTTime(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		end, err := parseSRTTime(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		blocks = append(blocks, srtBlock{start: start, end: end, text: strings.Join(lines[2:], "\n")})
	}
	return blocks, nil
}

func parseSRTTime(value string) (time.Duration, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, errors.New("invalid srt timestamp")
	}
	secParts := strings.Split(parts[2], ",")
	if len(secParts) != 2 {
		return 0, errors.New("invalid srt timestamp")
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	mins, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	secs, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, err
	}
	millis, err := strconv.Atoi(secParts[1])
	if err != nil {
		return 0, err
	}
	return time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute + time.Duration(secs)*time.Second + time.Duration(millis)*time.Millisecond, nil
}

func formatSRTTime(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	totalMillis := value.Milliseconds()
	millis := totalMillis % 1000
	totalSeconds := totalMillis / 1000
	seconds := totalSeconds % 60
	totalMinutes := totalSeconds / 60
	minutes := totalMinutes % 60
	hours := totalMinutes / 60
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func srtToText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := []string{}
	timestampLine := regexp.MustCompile(`\d{2}:\d{2}:\d{2},\d{3}\s*-->\s*\d{2}:\d{2}:\d{2},\d{3}`)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, err := strconv.Atoi(line); err == nil {
			continue
		}
		if timestampLine.MatchString(line) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, " "), nil
}
