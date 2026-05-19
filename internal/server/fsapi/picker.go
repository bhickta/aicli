package fsapi

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (h *Handler) PickDirectory(w http.ResponseWriter, r *http.Request) {
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
