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
	"time"
)

func (s *Service) prepareTranscriptFiles(ctx context.Context, videoPath, cacheDir, whisperModel string) (string, string, bool, error) {
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
		if err := s.transcribeVideo(ctx, videoPath, strings.TrimSuffix(cacheSRT, filepath.Ext(cacheSRT)), whisperModel); err != nil {
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

func (s *Service) transcribeVideo(ctx context.Context, videoPath, outputBase, whisperModel string) error {
	if strings.TrimSpace(s.tools.WhisperCLI) == "" {
		return errors.New("whisper-cli is not configured")
	}
	if whisperModel == "" {
		whisperModel = "large-v3"
	}
	args := []string{"-m", whisperModel, "-f", videoPath, "-osrt", "-otxt", "-of", outputBase}
	out, err := s.runner.CombinedOutput(ctx, s.tools.WhisperCLI, args...)
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
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
