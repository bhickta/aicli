package fsapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func (h *Handler) UploadFiles(w http.ResponseWriter, r *http.Request) {
	if h.dataDir == "" {
		writeError(w, http.StatusInternalServerError, errors.New("data directory is not configured"))
		return
	}
	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	uploadDir := filepath.Join(h.dataDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	uploaded := []UploadEntry{}
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
		entry, ok, err := saveUploadPart(uploadDir, part)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if !ok {
			continue
		}
		uploaded = append(uploaded, entry)
		rootSegments[rootSegment(entry.Name)] = true
	}
	if len(uploaded) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("no files uploaded"))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"files": uploaded, "root": uploadRoot(uploadDir, rootSegments)})
}

func saveUploadPart(uploadDir string, part multipartPart) (UploadEntry, bool, error) {
	defer part.Close()
	formName := part.FormName()
	if (formName != "file" && !strings.HasPrefix(formName, "file:")) || part.FileName() == "" {
		return UploadEntry{}, false, nil
	}
	uploadName := part.FileName()
	if strings.HasPrefix(formName, "file:") {
		uploadName = strings.TrimPrefix(formName, "file:")
	}
	relativeName := safeUploadRelativePath(uploadName)
	target := uniqueUploadPath(uploadDir, relativeName)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return UploadEntry{}, false, err
	}
	size, err := copyUploadPart(target, part)
	if err != nil {
		return UploadEntry{}, false, err
	}
	relativeURL, err := filepath.Rel(uploadDir, target)
	if err != nil {
		relativeURL = filepath.Base(target)
	}
	return UploadEntry{
		Name: filepath.ToSlash(relativeName),
		Path: target,
		URL:  "/uploads/" + escapeURLPath(filepath.ToSlash(relativeURL)),
		Size: size,
	}, true, nil
}

type multipartPart interface {
	FormName() string
	FileName() string
	io.Reader
	Close() error
}

func copyUploadPart(target string, part io.Reader) (int64, error) {
	dst, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, err
	}
	size, copyErr := io.Copy(dst, part)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(target)
		return 0, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return 0, closeErr
	}
	return size, nil
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
