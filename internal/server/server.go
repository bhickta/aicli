package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/analyze"
	"github.com/bhickta/aicli/internal/workflow/audio"
	"github.com/bhickta/aicli/internal/workflow/image"
	"github.com/bhickta/aicli/internal/workflow/news"
	"github.com/bhickta/aicli/internal/workflow/ocr"
	"github.com/bhickta/aicli/internal/workflow/recall"
	"github.com/bhickta/aicli/internal/workflow/video"
	"github.com/bhickta/aicli/web"
)

type Dependencies struct {
	Logger       *slog.Logger
	SettingsPath string
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
	s.mux.HandleFunc("GET /api/tools", s.tools)
	s.mux.HandleFunc("POST /api/chat", s.chat)
	s.mux.HandleFunc("POST /api/chat/stream", s.chatStream)
	s.mux.HandleFunc("POST /api/workflows/recall/run", s.runRecall)
	s.mux.HandleFunc("POST /api/workflows/image/run", s.runImage)
	s.mux.HandleFunc("POST /api/workflows/image/rename", s.runImageRename)
	s.mux.HandleFunc("POST /api/workflows/image/prune-refs", s.runImagePruneRefs)
	s.mux.HandleFunc("POST /api/workflows/news/run", s.runNews)
	s.mux.HandleFunc("POST /api/workflows/ocr/run", s.runOCR)
	s.mux.HandleFunc("POST /api/workflows/ocr/pdf", s.runPDFOCR)
	s.mux.HandleFunc("POST /api/workflows/analyze/run", s.runAnalyze)
	s.mux.HandleFunc("POST /api/workflows/video/info", s.runVideoInfo)
	s.mux.HandleFunc("POST /api/workflows/video/compress", s.runVideoCompress)
	s.mux.HandleFunc("POST /api/workflows/video/metadata/backup", s.runVideoMetadataBackup)
	s.mux.HandleFunc("POST /api/workflows/video/metadata/restore", s.runVideoMetadataRestore)
	s.mux.HandleFunc("POST /api/workflows/video/generate", s.runVideoGenerate)
	s.mux.HandleFunc("POST /api/workflows/audio/transcribe", s.runAudioTranscribe)
	s.mux.HandleFunc("POST /api/workflows/audio/analyze", s.runAudioAnalyze)
	s.mux.HandleFunc("GET /api/jobs", s.listJobs)
	s.mux.HandleFunc("POST /api/jobs", s.createJob)
	s.mux.HandleFunc("GET /api/jobs/", s.getJob)
	s.mux.Handle("/", http.FileServerFS(web.Static()))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.deps.Settings)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request) {
	var settings config.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Save(s.deps.SettingsPath, settings); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.deps.Settings = settings
	s.deps.Providers = provider.NewRegistry(settings.Providers)
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) listProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.deps.Settings.Providers})
}

func (s *Server) providerModels(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/providers/"), "/")
	if len(parts) != 2 || parts[1] != "models" {
		http.NotFound(w, r)
		return
	}
	p, ok := s.deps.Providers.Get(parts[0])
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	models, err := p.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

type fileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
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

func (s *Server) chat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		provider.ChatRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.ProviderID == "" {
		req.ProviderID = s.deps.Settings.DefaultProvider
	}
	p, ok := s.deps.Providers.Get(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	res, err := p.Chat(r.Context(), req.ChatRequest)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) chatStream(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		provider.ChatRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.ProviderID == "" {
		req.ProviderID = s.deps.Settings.DefaultProvider
	}
	p, ok := s.deps.Providers.Get(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	err := p.ChatStream(r.Context(), req.ChatRequest, func(chunk string) error {
		_, writeErr := fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(chunk, "\n", "\\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return writeErr
	})
	if err != nil {
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", strings.ReplaceAll(err.Error(), "\n", " "))
		return
	}
	_, _ = fmt.Fprint(w, "event: done\ndata: {}\n\n")
}

func (s *Server) tools(w http.ResponseWriter, r *http.Request) {
	checker := tool.Checker{}
	statuses := []tool.Status{
		checker.Check(r.Context(), "ffmpeg", s.deps.Settings.Tools.FFmpeg, "-version"),
		checker.Check(r.Context(), "ffprobe", s.deps.Settings.Tools.FFprobe, "-version"),
		checker.Check(r.Context(), "pdftoppm", s.deps.Settings.Tools.PDFToPPM, "-v"),
		checker.Check(r.Context(), "whisper-cli", s.deps.Settings.Tools.WhisperCLI, "--help"),
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": statuses})
}

func (s *Server) runRecall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		Notes      string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.ProviderID == "" {
		req.ProviderID = s.deps.Settings.DefaultProvider
	}
	p, ok := s.deps.Providers.Get(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}

	job := storage.Job{
		ID:     fmt.Sprintf("recall-%d", time.Now().UTC().UnixNano()),
		Type:   "recall",
		Status: "running",
		Input:  req.Notes,
	}
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	result, err := recall.New(p).Generate(r.Context(), recall.Request{Model: req.Model, Notes: req.Notes})
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		_ = s.deps.Store.UpdateJob(r.Context(), job)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	job.Status = "completed"
	job.Output = result.Triggers
	if err := s.deps.Store.UpdateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job, "result": result})
}

func (s *Server) runImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		image.Request
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("image", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := image.New(p).Run(r.Context(), req.Request)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runImageRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		image.RenameRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("image-rename", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := image.New(p).Rename(r.Context(), req.RenameRequest)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runImagePruneRefs(w http.ResponseWriter, r *http.Request) {
	var req image.PruneRefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("image-prune-refs", req.MarkdownPath)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := image.PruneRefs(req)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runNews(w http.ResponseWriter, r *http.Request) {
	var req news.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var p provider.Provider
	if req.UseLLM {
		selected, ok := s.providerFor(req.ProviderID)
		if !ok {
			writeError(w, http.StatusNotFound, errors.New("provider not found"))
			return
		}
		p = selected
	}
	job := s.newJob("news", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := news.New(p).Run(r.Context(), req)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runOCR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		ocr.Request
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("ocr", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := ocr.New(p).Run(r.Context(), req.Request)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runPDFOCR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		ocr.Request
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("pdf-ocr", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := ocr.New(
		p,
		ocr.WithPDFRenderer(s.deps.Settings.Tools, tool.ExecRunner{}),
	).RunPDF(r.Context(), req.Request)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		analyze.Request
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("analyze", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := analyze.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Run(r.Context(), req.Request)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runVideoInfo(w http.ResponseWriter, r *http.Request) {
	var req video.InfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-info", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).Info(r.Context(), req)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runVideoCompress(w http.ResponseWriter, r *http.Request) {
	var req video.CompressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-compress", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).Compress(r.Context(), req)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runVideoMetadataBackup(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-metadata-backup", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).BackupMetadata(r.Context(), req)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runVideoMetadataRestore(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-metadata-restore", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).RestoreMetadata(r.Context(), req)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runVideoGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		video.LLMRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("video-"+req.Mode, req.Title)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Generate(r.Context(), req.LLMRequest)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runAudioTranscribe(w http.ResponseWriter, r *http.Request) {
	var req audio.TranscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("audio-transcribe", req.Path)
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := audio.New(s.deps.Settings.Tools, tool.ExecRunner{}).Transcribe(r.Context(), req)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) runAudioAnalyze(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		audio.AnalyzeRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("audio-analyze", "")
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	result, err := audio.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Analyze(r.Context(), req.AnalyzeRequest)
	s.finishJob(w, r, job, result, err)
}

func (s *Server) providerFor(id string) (provider.Provider, bool) {
	if id == "" {
		id = s.deps.Settings.DefaultProvider
	}
	return s.deps.Providers.Get(id)
}

func (s *Server) newJob(jobType string, input string) storage.Job {
	return storage.Job{
		ID:     fmt.Sprintf("%s-%d", jobType, time.Now().UTC().UnixNano()),
		Type:   jobType,
		Status: "running",
		Input:  input,
	}
}

func (s *Server) finishJob(w http.ResponseWriter, r *http.Request, job storage.Job, result any, err error) {
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		_ = s.deps.Store.UpdateJob(r.Context(), job)
		writeError(w, http.StatusBadGateway, err)
		return
	}
	output, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		job.Status = "failed"
		job.Error = marshalErr.Error()
		_ = s.deps.Store.UpdateJob(r.Context(), job)
		writeError(w, http.StatusInternalServerError, marshalErr)
		return
	}
	job.Status = "completed"
	job.Output = string(output)
	if err := s.deps.Store.UpdateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"job": job, "result": result})
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.deps.Store.ListJobs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var job storage.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if job.ID == "" || job.Type == "" {
		writeError(w, http.StatusBadRequest, errors.New("job id and type are required"))
		return
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	job, err := s.deps.Store.GetJob(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}
