package core

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/storage"
)

type Dependencies struct {
	Logger      *slog.Logger
	Store       storage.Store
	Settings    func() config.Settings
	ProviderFor func(id string) (provider.Provider, bool)
}

type Runtime struct {
	logger      *slog.Logger
	store       storage.Store
	settings    func() config.Settings
	providerFor func(id string) (provider.Provider, bool)
	cancelMu    sync.Mutex
	cancelers   map[string]context.CancelFunc
}

func New(deps Dependencies) *Runtime {
	return &Runtime{
		logger:      deps.Logger,
		store:       deps.Store,
		settings:    deps.Settings,
		providerFor: deps.ProviderFor,
		cancelers:   map[string]context.CancelFunc{},
	}
}

func (r *Runtime) Settings() config.Settings {
	return r.settings()
}

func (r *Runtime) ProviderFor(id string) (provider.Provider, bool) {
	return r.providerFor(id)
}

func (r *Runtime) registerCancel(jobID string, cancel context.CancelFunc) {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	r.cancelers[jobID] = cancel
}

func (r *Runtime) unregisterCancel(jobID string) {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	delete(r.cancelers, jobID)
}

func (r *Runtime) cancelFunc(jobID string) (context.CancelFunc, bool) {
	r.cancelMu.Lock()
	defer r.cancelMu.Unlock()
	cancel, ok := r.cancelers[jobID]
	return cancel, ok
}
