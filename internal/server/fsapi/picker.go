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

func (h *Handler) PickFile(w http.ResponseWriter, r *http.Request) {
	initial := r.URL.Query().Get("path")
	if initial == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		initial = home
	}
	path, err := systemFilePicker(r.Context(), initial)
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

func systemFilePicker(ctx context.Context, initial string) (string, error) {
	initial, _ = filepath.Abs(initial)
	if info, err := os.Stat(initial); err == nil && info.IsDir() {
		initial = ensureTrailingSeparator(initial)
	}
	if path, err := runFilePicker(ctx, "zenity", "--file-selection", "--title", "Choose file", "--filename", initial); err == nil {
		return path, nil
	}
	if path, err := runFilePicker(ctx, "yad", "--file", "--title", "Choose file", "--filename", initial); err == nil {
		return path, nil
	}
	if path, err := runFilePicker(ctx, "kdialog", "--getopenfilename", initial, "--title", "Choose file"); err == nil {
		return path, nil
	}
	return "", errors.New("system file picker is unavailable or was cancelled")
}

func runDirectoryPicker(ctx context.Context, name string, args ...string) (string, error) {
	return runPathPicker(ctx, name, true, args...)
}

func runFilePicker(ctx context.Context, name string, args ...string) (string, error) {
	return runPathPicker(ctx, name, false, args...)
}

func runPathPicker(ctx context.Context, name string, wantDir bool, args ...string) (string, error) {
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
	if wantDir && !info.IsDir() {
		return "", errors.New("selected path is not a folder")
	}
	if !wantDir && info.IsDir() {
		return "", errors.New("selected path is not a file")
	}
	return abs, nil
}

func ensureTrailingSeparator(path string) string {
	if path == "" || strings.HasSuffix(path, string(filepath.Separator)) {
		return path
	}
	return path + string(filepath.Separator)
}
