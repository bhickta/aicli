package zettelapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
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
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	job := core.NewJob("zettel-index", req.VaultPath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Index(ctx, req, progress)
	})
}

func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	var req zettel.SuggestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	job := core.NewJob("zettel-suggest", req.ActivePath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Suggest(ctx, req, progress)
	})
}

func (h *Handler) propose(w http.ResponseWriter, r *http.Request) {
	var req zettel.ProposeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	job := core.NewJob("zettel-propose", req.ActivePath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Propose(ctx, req, progress)
	})
}

func (h *Handler) apply(w http.ResponseWriter, r *http.Request) {
	var req zettel.ApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	job := core.NewJob("zettel-apply", req.Proposal.ActivePath)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Apply(ctx, req, progress)
	})
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	var req zettel.RollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	job := core.NewJob("zettel-rollback", req.JobID)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Rollback(ctx, req, progress)
	})
}

func (h *Handler) service(options zettel.Options) (*zettel.Service, error) {
	requestedEmbeddingProviderID := strings.TrimSpace(options.EmbeddingProviderID)
	options = zettel.NormalizeOptions(options)
	p, ok := h.runtime.ProviderFor(options.ProviderID)
	if !ok {
		return nil, errors.New("provider not found")
	}
	embeddingProviderID := options.EmbeddingProviderID
	if requestedEmbeddingProviderID == "" && !supportsEmbeddings(p) {
		if fallback := strings.TrimSpace(h.runtime.Settings().DefaultProvider); fallback != "" {
			embeddingProviderID = fallback
		}
	}
	embeddingProvider, ok := h.runtime.ProviderFor(embeddingProviderID)
	if !ok {
		return nil, errors.New("embedding provider not found")
	}
	return zettel.NewWithEmbedding(p, embeddingProvider), nil
}

type embeddingProvider interface {
	Embeddings(context.Context, provider.EmbeddingRequest) (provider.EmbeddingResponse, error)
}

func supportsEmbeddings(p provider.Provider) bool {
	_, ok := p.(embeddingProvider)
	return ok
}
