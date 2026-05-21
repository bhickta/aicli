package inbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

func writeDestinationNotes(v vault, options Options, destinationAfter map[string]string) error {
	paths := make([]string, 0, len(destinationAfter))
	for path := range destinationAfter {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		abs, err := v.NotePath(path, options)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("create destination note folder %s: %w", path, err)
		}
		if err := os.WriteFile(abs, []byte(notetext.EnsureTrailingNewline(destinationAfter[path])), 0o600); err != nil {
			return fmt.Errorf("write destination note %s: %w", path, err)
		}
	}
	return nil
}

func readDestinationNote(v vault, options Options, path string) (string, error) {
	abs, err := v.NotePath(path, options)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read destination note %s: %w", path, err)
	}
	return string(content), nil
}

func moveInboxSourceToProcessed(v vault, options Options, sourcePath string) (string, error) {
	return moveInboxSourceToFolder(v, options, sourcePath, "_processed")
}

func moveInboxSourceToPending(v vault, options Options, sourcePath string) (string, error) {
	return moveInboxSourceToFolder(v, options, sourcePath, "_pending")
}

func moveInboxSourceToFolder(v vault, options Options, sourcePath string, folder string) (string, error) {
	inbox := strings.Trim(filepath.ToSlash(filepath.Clean(options.InboxFolder)), "/")
	source := strings.Trim(filepath.ToSlash(filepath.Clean(sourcePath)), "/")
	relInside := strings.TrimPrefix(source, inbox)
	relInside = strings.Trim(relInside, "/")
	if relInside == "" {
		relInside = filepath.Base(source)
	}
	processedRel := filepath.ToSlash(filepath.Join(inbox, folder, time.Now().Format("2006-01-02"), relInside))
	processedAbs, err := v.Abs(processedRel)
	if err != nil {
		return "", err
	}
	processedAbs = uniquePath(processedAbs)
	sourceAbs, err := v.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(processedAbs), 0o755); err != nil {
		return "", fmt.Errorf("create processed folder: %w", err)
	}
	if err := os.Rename(sourceAbs, processedAbs); err != nil {
		return "", fmt.Errorf("move processed source: %w", err)
	}
	rel, err := v.Rel(processedAbs)
	if err != nil {
		return "", err
	}
	return rel, nil
}

func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}
