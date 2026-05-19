package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/analyze"
	"github.com/bhickta/aicli/internal/workflow/ocr"
)

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
