package app

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider/registry"
	"github.com/bhickta/aicli/internal/server"
	"github.com/bhickta/aicli/internal/storage"
)

type Options struct {
	Host        string
	Port        int
	DataDir     string
	ConfigPath  string
	OpenBrowser bool
}

type App struct {
	db      *sql.DB
	handler http.Handler
}

func New(opts Options, logger *slog.Logger) (*App, error) {
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return nil, err
	}

	settings, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, err
	}

	db, err := storage.OpenSQLite(filepath.Join(opts.DataDir, "aicli.db"))
	if err != nil {
		return nil, err
	}

	store := storage.NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	interrupted, err := store.MarkRunningJobsInterrupted(context.Background(), "interrupted by AICLI restart")
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if interrupted > 0 && logger != nil {
		logger.Info("marked interrupted jobs", "count", interrupted)
	}

	providers := registry.New(settings.Providers, settings.Tools)
	handler := server.New(server.Dependencies{
		Logger:         logger,
		SettingsPath:   opts.ConfigPath,
		DataDir:        opts.DataDir,
		Settings:       settings,
		Store:          store,
		Providers:      providers,
		ExecutionToken: os.Getenv("AICLI_SERVICE_TOKEN"),
	})

	return &App{db: db, handler: handler}, nil
}

func (a *App) Handler() http.Handler {
	return a.handler
}

func (a *App) Close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}
