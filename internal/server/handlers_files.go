package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}

		name := safeUploadName(part.FileName())
		target := uniqueUploadPath(uploadDir, name)
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
		uploaded = append(uploaded, uploadEntry{
			Name: name,
			Path: target,
			URL:  "/uploads/" + url.PathEscape(filepath.Base(target)),
			Size: size,
		})
	}
	if len(uploaded) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no files uploaded"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"files": uploaded})
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
