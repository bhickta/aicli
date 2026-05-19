package core

import (
	"log/slog"

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
}

func New(deps Dependencies) *Runtime {
	return &Runtime{
		logger:      deps.Logger,
		store:       deps.Store,
		settings:    deps.Settings,
		providerFor: deps.ProviderFor,
	}
}

func (r *Runtime) Settings() config.Settings {
	return r.settings()
}

func (r *Runtime) ProviderFor(id string) (provider.Provider, bool) {
	return r.providerFor(id)
}
