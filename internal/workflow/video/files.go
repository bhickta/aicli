package video

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func rawVideoFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if isVideoFile(path) {
			return []string{path}, nil
		}
		return nil, nil
	}
	files := []string{}
	err = filepath.WalkDir(path, func(p string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == ".aicli_cache" || name == "course" || name == "trash" {
				return filepath.SkipDir
			}
			return nil
		}
		name := strings.ToLower(filepath.Base(p))
		if isVideoFile(p) && !strings.Contains(name, "slideshow") && !strings.Contains(name, "merged") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func isVideoFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v":
		return true
	default:
		return false
	}
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func filepathExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}
