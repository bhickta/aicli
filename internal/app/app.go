package app

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

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

	db, err := sql.Open("sqlite", filepath.Join(opts.DataDir, "aicli.db"))
	if err != nil {
		return nil, err
	}

	store := storage.NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	providers := registry.New(settings.Providers)
	handler := server.New(server.Dependencies{
		Logger:       logger,
		SettingsPath: opts.ConfigPath,
		DataDir:      opts.DataDir,
		Settings:     settings,
		Store:        store,
		Providers:    providers,
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
