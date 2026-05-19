package server

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	writeJSON(w, http.StatusOK, map[string]any{"path": abs, "entries": fileEntries(abs, entries)})
}

func fileEntries(abs string, entries []os.DirEntry) []fileEntry {
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
	return out
}
