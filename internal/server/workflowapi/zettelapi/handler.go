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
	service, err := h.serviceFor(req.Options, providerNeeds{embedding: true})
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-index", req.VaultPath, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.Index(ctx, req, progress)
	})
}

func (h *Handler) rollback(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[zettel.RollbackRequest](w, r)
	if !ok {
		return
	}
	service, err := h.serviceFor(req.Options, providerNeeds{})
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
	service, err := h.serviceFor(req.Options, providerNeeds{merge: true, embedding: true})
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	h.runtime.StartJob(w, r, "zettel-inbox-merge", req.InboxFolder, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		return service.InboxMerge(ctx, req, progress)
	})
}

type providerNeeds struct {
	merge     bool
	embedding bool
}

func (h *Handler) serviceFor(options zettel.Options, needs providerNeeds) (*zettel.Service, error) {
	requestedEmbeddingProviderID := strings.TrimSpace(options.EmbeddingProviderID)
	options = zettel.NormalizeOptions(options)
	var mergeProvider provider.Provider
	var embeddingProvider provider.Provider
	if needs.merge {
		var ok bool
		mergeProvider, ok = h.runtime.ProviderFor(options.MergeProviderID)
		if !ok {
			return nil, errors.New("merge provider not found")
		}
	}
	if needs.embedding {
		embeddingProviderID := options.EmbeddingProviderID
		var ok bool
		embeddingProvider, ok = h.runtime.ProviderFor(embeddingProviderID)
		if !ok {
			return nil, errors.New("embedding provider not found")
		}
		if requestedEmbeddingProviderID == "" && !supportsEmbeddings(embeddingProvider) {
			if fallback := strings.TrimSpace(h.runtime.Settings().DefaultProvider); fallback != "" && fallback != embeddingProviderID {
				if fallbackProvider, ok := h.runtime.ProviderFor(fallback); ok && supportsEmbeddings(fallbackProvider) {
					embeddingProvider = fallbackProvider
				}
			}
		}
		if !supportsEmbeddings(embeddingProvider) {
			return nil, errors.New("embedding provider does not support embeddings")
		}
	}
	return zettel.NewWithProviders(
		mergeProvider,
		embeddingProvider,
	).WithDataDir(h.runtime.DataDir()), nil
}

type embeddingProvider interface {
	Embeddings(context.Context, provider.EmbeddingRequest) (provider.EmbeddingResponse, error)
}

func supportsEmbeddings(p provider.Provider) bool {
	_, ok := p.(embeddingProvider)
	return ok
}
