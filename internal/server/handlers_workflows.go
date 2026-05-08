package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/analyze"
	"github.com/bhickta/aicli/internal/workflow/audio"
	"github.com/bhickta/aicli/internal/workflow/image"
	"github.com/bhickta/aicli/internal/workflow/news"
	"github.com/bhickta/aicli/internal/workflow/ocr"
	"github.com/bhickta/aicli/internal/workflow/recall"
	"github.com/bhickta/aicli/internal/workflow/video"
)

func (s *Server) registerWorkflowRoutes() {
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
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}

	job := s.newJob("recall", req.Notes)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("generating recall triggers", 2, 4)
		result, err := recall.New(p).Generate(ctx, recall.Request{Model: req.Model, Notes: req.Notes})
		progress("saving triggers", 3, 4)
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("analyzing image with vision model", 2, 4)
		result, err := image.New(p).Run(ctx, req.Request)
		progress("saving result", 3, 4)
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("planning safe rename", 2, 4)
		result, err := image.New(p).Rename(ctx, req.RenameRequest)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runImagePruneRefs(w http.ResponseWriter, r *http.Request) {
	var req image.PruneRefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("image-prune-refs", req.MarkdownPath)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("checking referenced assets", 2, 4)
		result, err := image.PruneRefs(req)
		progress("saving result", 3, 4)
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("loading and deduplicating news", 2, 4)
		result, err := news.New(p).Run(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("extracting images from ZIP", 2, 5)
		progress("OCR pages in parallel", 3, 5)
		result, err := ocr.New(p).Run(ctx, req.Request)
		progress("assembling markdown", 4, 5)
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress(fmt.Sprintf("rendering PDF pages with %d worker(s)", req.RenderWorkers), 2, 5)
		result, err := ocr.New(
			p,
			ocr.WithPDFRenderer(s.deps.Settings.Tools, tool.ExecRunner{}),
		).RunPDFWithProgress(ctx, req.Request, func(stage string) {
			progress(stage, 3, 5)
		})
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("rendering and reading PDF", 2, 5)
		progress("analyzing OCR text", 3, 5)
		result, err := analyze.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Run(ctx, req.Request)
		progress("saving analysis", 4, 5)
		return result, err
	})
}

func (s *Server) runVideoInfo(w http.ResponseWriter, r *http.Request) {
	var req video.InfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-info", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("probing media metadata", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).Info(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runVideoCompress(w http.ResponseWriter, r *http.Request) {
	var req video.CompressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-compress", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("compressing video", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).Compress(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runVideoMetadataBackup(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-metadata-backup", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("backing up metadata", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).BackupMetadata(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runVideoMetadataRestore(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-metadata-restore", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("restoring metadata", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).RestoreMetadata(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("generating video workflow text", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Generate(ctx, req.LLMRequest)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runAudioTranscribe(w http.ResponseWriter, r *http.Request) {
	var req audio.TranscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("audio-transcribe", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("transcribing audio", 2, 4)
		result, err := audio.New(s.deps.Settings.Tools, tool.ExecRunner{}).Transcribe(ctx, req)
		progress("saving transcript", 3, 4)
		return result, err
	})
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
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("analyzing audio text", 2, 4)
		result, err := audio.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Analyze(ctx, req.AnalyzeRequest)
		progress("saving analysis", 3, 4)
		return result, err
	})
}
