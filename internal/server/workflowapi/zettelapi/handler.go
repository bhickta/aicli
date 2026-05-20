package zettelapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/workflow/zettel"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/zettel/index", h.index)
	mux.HandleFunc("POST /api/workflows/zettel/suggest", h.suggest)
	mux.HandleFunc("POST /api/workflows/zettel/propose", h.propose)
	mux.HandleFunc("POST /api/workflows/zettel/apply", h.apply)
	mux.HandleFunc("POST /api/workflows/zettel/rollback", h.rollback)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	var req zettel.IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("zettel-index", req.VaultPath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return zettel.New(p).Index(ctx, req, progress)
	})
}

func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	var req zettel.SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("zettel-suggest", req.ActivePath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return zettel.New(p).Suggest(ctx, req, progress)
	})
}

func (h *Handler) propose(w http.ResponseWriter, r *http.Request) {
	var req zettel.ProposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("zettel-propose", req.ActivePath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return zettel.New(p).Propose(ctx, req, progress)
	})
}

func (h *Handler) apply(w http.ResponseWriter, r *http.Request) {
	var req zettel.ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("zettel-apply", req.Proposal.ActivePath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return zettel.New(p).Apply(ctx, req, progress)
	})
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	var req zettel.RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("zettel-rollback", req.JobID)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return zettel.New(p).Rollback(ctx, req, progress)
	})
}
