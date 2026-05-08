package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/web"
)

type Dependencies struct {
	Logger       *slog.Logger
	SettingsPath string
	DataDir      string
	Settings     config.Settings
	Store        storage.Store
	Providers    *provider.Registry
}

type Server struct {
	deps Dependencies
	mux  *http.ServeMux
}

func New(deps Dependencies) http.Handler {
	s := &Server{deps: deps, mux: http.NewServeMux()}
	s.routes()
	return s.withLogging(s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.health)
	s.mux.HandleFunc("GET /api/settings", s.getSettings)
	s.mux.HandleFunc("PUT /api/settings", s.updateSettings)
	s.mux.HandleFunc("GET /api/providers", s.listProviders)
	s.mux.HandleFunc("GET /api/providers/", s.providerModels)
	s.mux.HandleFunc("GET /api/fs/list", s.listFiles)
	s.mux.HandleFunc("POST /api/fs/upload", s.uploadFiles)
	if s.deps.DataDir != "" {
		uploads := http.Dir(filepath.Join(s.deps.DataDir, "uploads"))
		s.mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(uploads)))
	}
	s.mux.HandleFunc("GET /api/tools", s.tools)
	s.mux.HandleFunc("POST /api/chat", s.chat)
	s.mux.HandleFunc("POST /api/chat/stream", s.chatStream)
	s.registerWorkflowRoutes()
	s.mux.HandleFunc("GET /api/jobs", s.listJobs)
	s.mux.HandleFunc("POST /api/jobs", s.createJob)
	s.mux.HandleFunc("GET /api/jobs/", s.getJob)
	s.mux.Handle("/", http.FileServerFS(web.Static()))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		if s.deps.Logger != nil && shouldLogRequest(r) {
			s.deps.Logger.Info(
				"http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"duration_ms", time.Since(started).Milliseconds(),
			)
		}
	})
}

func shouldLogRequest(r *http.Request) bool {
	return !(r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/jobs/"))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
