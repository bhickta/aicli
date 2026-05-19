package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type fileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type uploadEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("path")
	if target == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		target = home
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out := make([]fileEntry, 0, len(entries)+1)
	parent := filepath.Dir(abs)
	if parent != abs {
		out = append(out, fileEntry{Name: "..", Path: parent, IsDir: true})
	}
	for _, entry := range entries {
		out = append(out, fileEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(abs, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name == ".." {
			return true
		}
		if out[j].Name == ".." {
			return false
		}
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	writeJSON(w, http.StatusOK, map[string]any{"path": abs, "entries": out})
}

func (s *Server) pickDirectory(w http.ResponseWriter, r *http.Request) {
	initial := r.URL.Query().Get("path")
	if initial == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		initial = home
	}
	path, err := systemDirectoryPicker(r.Context(), initial)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) uploadFiles(w http.ResponseWriter, r *http.Request) {
	if s.deps.DataDir == "" {
		writeError(w, http.StatusInternalServerError, errors.New("data directory is not configured"))
		return
	}
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	uploadDir := filepath.Join(s.deps.DataDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	uploaded := []uploadEntry{}
	rootSegments := map[string]bool{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		formName := part.FormName()
		if (formName != "file" && !strings.HasPrefix(formName, "file:")) || part.FileName() == "" {
			_ = part.Close()
			continue
		}

		uploadName := part.FileName()
		if strings.HasPrefix(formName, "file:") {
			uploadName = strings.TrimPrefix(formName, "file:")
		}
		relativeName := safeUploadRelativePath(uploadName)
		rootSegments[rootSegment(relativeName)] = true
		target := uniqueUploadPath(uploadDir, relativeName)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			_ = part.Close()
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			_ = part.Close()
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		size, copyErr := io.Copy(dst, part)
		closeErr := dst.Close()
		_ = part.Close()
		if copyErr != nil {
			_ = os.Remove(target)
			writeError(w, http.StatusInternalServerError, copyErr)
			return
		}
		if closeErr != nil {
			_ = os.Remove(target)
			writeError(w, http.StatusInternalServerError, closeErr)
			return
		}
		relativeURL, err := filepath.Rel(uploadDir, target)
		if err != nil {
			relativeURL = filepath.Base(target)
		}
		uploaded = append(uploaded, uploadEntry{
			Name: filepath.ToSlash(relativeName),
			Path: target,
			URL:  "/uploads/" + escapeURLPath(filepath.ToSlash(relativeURL)),
			Size: size,
		})
	}
	if len(uploaded) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no files uploaded"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"files": uploaded, "root": uploadRoot(uploadDir, rootSegments)})
}

func safeUploadName(name string) string {
	name = filepath.Base(name)
	name = strings.TrimSpace(name)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return fmt.Sprintf("upload-%d", time.Now().UTC().UnixNano())
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", "\x00", "")
	return replacer.Replace(name)
}

func safeUploadRelativePath(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	cleaned := path.Clean(name)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." || cleaned == "" {
		return safeUploadName(name)
	}
	parts := strings.Split(cleaned, "/")
	safeParts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			continue
		}
		safeParts = append(safeParts, safeUploadName(part))
	}
	if len(safeParts) == 0 {
		return safeUploadName(name)
	}
	return filepath.Join(safeParts...)
}

func uniqueUploadPath(uploadDir, name string) string {
	target := filepath.Join(uploadDir, name)
	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return target
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		candidate := filepath.Join(uploadDir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

func rootSegment(relativeName string) string {
	parts := strings.Split(filepath.ToSlash(relativeName), "/")
	if len(parts) > 1 {
		return parts[0]
	}
	return ""
}

func uploadRoot(uploadDir string, roots map[string]bool) string {
	if len(roots) == 1 {
		for root := range roots {
			if root != "" {
				return filepath.Join(uploadDir, root)
			}
		}
	}
	return uploadDir
}

func escapeURLPath(relativePath string) string {
	parts := strings.Split(relativePath, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func systemDirectoryPicker(ctx context.Context, initial string) (string, error) {
	initial, _ = filepath.Abs(initial)
	if info, err := os.Stat(initial); err == nil && !info.IsDir() {
		initial = filepath.Dir(initial)
	}
	if path, err := runDirectoryPicker(ctx, "zenity", "--file-selection", "--directory", "--title", "Choose folder", "--filename", ensureTrailingSeparator(initial)); err == nil {
		return path, nil
	}
	if path, err := runDirectoryPicker(ctx, "yad", "--file", "--directory", "--title", "Choose folder", "--filename", ensureTrailingSeparator(initial)); err == nil {
		return path, nil
	}
	if path, err := runDirectoryPicker(ctx, "kdialog", "--getexistingdirectory", initial, "--title", "Choose folder"); err == nil {
		return path, nil
	}
	return "", errors.New("system folder picker is unavailable or was cancelled")
}

func runDirectoryPicker(ctx context.Context, name string, args ...string) (string, error) {
	if _, err := exec.LookPath(name); err != nil {
		return "", err
	}
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	selected := strings.TrimSpace(string(out))
	if selected == "" {
		return "", errors.New("no folder selected")
	}
	abs, err := filepath.Abs(selected)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("selected path is not a folder")
	}
	return abs, nil
}

func ensureTrailingSeparator(path string) string {
	if path == "" || strings.HasSuffix(path, string(filepath.Separator)) {
		return path
	}
	return path + string(filepath.Separator)
}
