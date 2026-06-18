package lecture

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type noteInput struct {
	Rel     string
	Content string
}

func collectNotes(req Request) ([]noteInput, int, int, error) {
	vaultRoot, err := cleanDir(req.VaultPath, "vault path")
	if err != nil {
		return nil, 0, 0, err
	}
	source := strings.TrimSpace(req.SourcePath)
	if source == "" {
		source = vaultRoot
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(vaultRoot, source)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("resolve source path: %w", err)
	}
	if err := ensureInside(vaultRoot, source); err != nil {
		return nil, 0, 0, err
	}

	info, err := os.Stat(source)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("stat source path: %w", err)
	}
	paths := []string{}
	if info.IsDir() {
		err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if shouldSkipDir(entry.Name(), path != source) {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".md") {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			return nil, 0, 0, fmt.Errorf("scan notes: %w", err)
		}
	} else if strings.EqualFold(filepath.Ext(source), ".md") {
		paths = append(paths, source)
	} else {
		return nil, 0, 0, fmt.Errorf("source path is not a markdown note or folder: %s", source)
	}
	sort.Strings(paths)

	maxNotes := req.MaxNotes
	if maxNotes <= 0 {
		maxNotes = 25
	}
	maxChars := req.MaxInputChars
	if maxChars <= 0 {
		maxChars = 120000
	}

	notes := make([]noteInput, 0, min(maxNotes, len(paths)))
	totalChars := 0
	skipped := 0
	for _, path := range paths {
		if len(notes) >= maxNotes {
			skipped++
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("read note %s: %w", path, err)
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			skipped++
			continue
		}
		if totalChars+len(content) > maxChars {
			remaining := maxChars - totalChars
			if remaining < 1200 {
				skipped++
				continue
			}
			content = content[:remaining]
		}
		rel, err := filepath.Rel(vaultRoot, path)
		if err != nil {
			return nil, 0, 0, err
		}
		notes = append(notes, noteInput{Rel: filepath.ToSlash(rel), Content: content})
		totalChars += len(content)
		if totalChars >= maxChars {
			skipped += len(paths) - len(notes)
			break
		}
	}
	if len(notes) == 0 {
		return nil, 0, 0, errors.New("no markdown note content found")
	}
	return notes, skipped, totalChars, nil
}

func cleanDir(path string, label string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", label, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", label, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory: %s", label, abs)
	}
	return abs, nil
}

func ensureInside(root string, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("relativize source path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("source path is outside vault: %s", path)
	}
	return nil
}

func shouldSkipDir(name string, nested bool) bool {
	if !nested {
		return false
	}
	return strings.HasPrefix(name, ".") || name == "node_modules" || name == "_processed" || name == "_pending"
}
