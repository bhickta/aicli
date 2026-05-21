package zettelapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	zettel "github.com/bhickta/aicli/internal/workflow/zettel/api"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/zettel/notes", h.notes)
	mux.HandleFunc("POST /api/workflows/zettel/index", h.index)
	mux.HandleFunc("POST /api/workflows/zettel/suggest", h.suggest)
	mux.HandleFunc("POST /api/workflows/zettel/propose", h.propose)
	mux.HandleFunc("POST /api/workflows/zettel/apply", h.apply)
	mux.HandleFunc("POST /api/workflows/zettel/rollback", h.rollback)
	mux.HandleFunc("POST /api/workflows/zettel/inbox-merge", h.inboxMerge)
}

func (h *Handler) notes(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.ListNotesRequest](w, r)
	if !ok {
		return
	}
	resp, err := zettel.ListNotes(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.IndexRequest](w, r)
	if !ok {
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-index", req.VaultPath, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Index(ctx, req, progress)
	})
}

func (h *Handler) suggest(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.SuggestRequest](w, r)
	if !ok {
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-suggest", req.ActivePath, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Suggest(ctx, req, progress)
	})
}

func (h *Handler) propose(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.ProposeRequest](w, r)
	if !ok {
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-propose", req.ActivePath, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Propose(ctx, req, progress)
	})
}

func (h *Handler) apply(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.ApplyRequest](w, r)
	if !ok {
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-apply", req.Proposal.ActivePath, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Apply(ctx, req, progress)
	})
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.RollbackRequest](w, r)
	if !ok {
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-rollback", req.JobID, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Rollback(ctx, req, progress)
	})
}

func (h *Handler) inboxMerge(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.InboxMergeRequest](w, r)
	if !ok {
		return
	}
	service, err := h.service(req.Options)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-inbox-merge", req.InboxFolder, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.InboxMerge(ctx, req, progress)
	})
}

func (h *Handler) service(options zettel.Options) (*zettel.Service, error) {
	requestedEmbeddingProviderID := strings.TrimSpace(options.EmbeddingProviderID)
	options = zettel.NormalizeOptions(options)
	candidateProvider, ok := h.runtime.ProviderFor(options.CandidateProviderID)
	if !ok {
		return nil, errors.New("candidate judge provider not found")
	}
	mergeProvider, ok := h.runtime.ProviderFor(options.MergeProviderID)
	if !ok {
		return nil, errors.New("merge provider not found")
	}
	validationProvider, ok := h.runtime.ProviderFor(options.ValidationProviderID)
	if !ok {
		return nil, errors.New("validation judge provider not found")
	}
	embeddingProviderID := options.EmbeddingProviderID
	if requestedEmbeddingProviderID == "" && !supportsEmbeddings(candidateProvider) {
		if fallback := strings.TrimSpace(h.runtime.Settings().DefaultProvider); fallback != "" {
			embeddingProviderID = fallback
		}
	}
	embeddingProvider, ok := h.runtime.ProviderFor(embeddingProviderID)
	if !ok {
		return nil, errors.New("embedding provider not found")
	}
	return zettel.NewWithProviders(candidateProvider, mergeProvider, validationProvider, embeddingProvider), nil
}

type embeddingProvider interface {
	Embeddings(context.Context, provider.EmbeddingRequest) (provider.EmbeddingResponse, error)
}

func supportsEmbeddings(p provider.Provider) bool {
	_, ok := p.(embeddingProvider)
	return ok
}
