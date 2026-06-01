package video

import (
	"crypto/sha1"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

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

func prepareCourseDirs(targetDir string, outputDir string, workDir string) (string, string, string, error) {
	courseDir := outputDir
	if strings.TrimSpace(courseDir) == "" {
		courseDir = filepath.Join(targetDir, "Course")
	}
	if err := os.MkdirAll(courseDir, 0o755); err != nil {
		return "", "", "", err
	}

	cacheDir := filepath.Join(targetDir, ".aicli_cache")
	if strings.TrimSpace(workDir) != "" {
		root, err := filepath.Abs(workDir)
		if err != nil {
			return "", "", "", err
		}
		cacheDir = filepath.Join(root, cacheCourseFolder(targetDir))
	}
	slidesDir := filepath.Join(cacheDir, "slideshows")
	if err := os.MkdirAll(slidesDir, 0o755); err != nil {
		return "", "", "", err
	}
	return courseDir, cacheDir, slidesDir, nil
}

func cacheCourseFolder(targetDir string) string {
	name := sanitizeCourseName(filepath.Base(targetDir))
	if name == "" {
		name = "course"
	}
	sum := sha1.Sum([]byte(targetDir))
	return name + "-" + hex.EncodeToString(sum[:])[:12]
}
