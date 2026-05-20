package tool

import (
	"os"
	"strings"
)

func TempOutputPath(pattern string) (string, func(), error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func FinalOutput(outputPath string, raw []byte) string {
	if data, err := os.ReadFile(outputPath); err == nil {
		if text := strings.TrimSpace(string(data)); text != "" {
			return text
		}
	}
	return strings.TrimSpace(string(raw))
}

func LimitedOutput(raw []byte, max int) string {
	text := strings.TrimSpace(string(raw))
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}
