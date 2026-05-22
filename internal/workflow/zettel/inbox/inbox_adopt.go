package inbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func adoptInboxDestinationPath(v vault, options Options, sourcePath string) (string, bool, error) {
	inbox := strings.Trim(filepath.ToSlash(filepath.Clean(options.InboxFolder)), "/")
	source := strings.Trim(filepath.ToSlash(filepath.Clean(sourcePath)), "/")
	relInside := strings.Trim(strings.TrimPrefix(source, inbox), "/")
	if relInside == "" {
		relInside = filepath.Base(source)
	}
	relInside = filepath.ToSlash(relInside)
	category := matchedRootCategory(v, options, relInside)
	parts := []string{options.RootFolder}
	if category != "" {
		parts = append(parts, category)
	}
	parts = append(parts, filepath.FromSlash(relInside))
	candidateRel := filepath.ToSlash(filepath.Join(parts...))
	abs, err := v.NotePath(candidateRel, options)
	if err != nil {
		return "", false, err
	}
	created := true
	if _, err := os.Stat(abs); err == nil {
		abs = uniquePath(abs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", false, fmt.Errorf("stat adopted destination: %w", err)
	}
	rel, err := v.Rel(abs)
	if err != nil {
		return "", false, err
	}
	return rel, created, nil
}

func matchedRootCategory(v vault, options Options, relInside string) string {
	rootAbs, err := v.Abs(options.RootFolder)
	if err != nil {
		return ""
	}
	entries, err := os.ReadDir(rootAbs)
	if err != nil {
		return ""
	}
	lowerSource := strings.ToLower(strings.ReplaceAll(relInside, "_", " "))
	best := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		lowerName := strings.ToLower(strings.ReplaceAll(name, "_", " "))
		if lowerName == "" || !strings.Contains(lowerSource, lowerName) {
			continue
		}
		if len(name) > len(best) {
			best = name
		}
	}
	return best
}
