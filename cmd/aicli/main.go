package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bhickta/aicli/internal/app"
	"github.com/bhickta/aicli/internal/config"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var opts app.Options
	var showVersion bool

	fs := flag.NewFlagSet("aicli", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&opts.Host, "host", "127.0.0.1", "host interface to bind")
	fs.IntVar(&opts.Port, "port", 8765, "port to serve")
	fs.StringVar(&opts.DataDir, "data-dir", defaultDataDir(), "directory for local app state")
	fs.StringVar(&opts.ConfigPath, "config", "", "settings file path")
	fs.BoolVar(&opts.OpenBrowser, "open", false, "print the app URL after startup")
	fs.BoolVar(&showVersion, "version", false, "print version and exit")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if showVersion {
		fmt.Fprintln(os.Stdout, "aicli dev")
		return 0
	}

	if opts.ConfigPath == "" {
		opts.ConfigPath = filepath.Join(opts.DataDir, "settings.json")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	application, err := app.New(opts, logger)
	if err != nil {
		logger.Error("initialize app", "error", err)
		return 1
	}
	defer application.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := net.JoinHostPort(opts.Host, fmt.Sprintf("%d", opts.Port))
	server := &http.Server{
		Addr:              addr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting aicli", "url", "http://"+addr, "data_dir", opts.DataDir)
		if opts.OpenBrowser {
			fmt.Fprintf(os.Stdout, "AICLI: http://%s\n", addr)
		}
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown server", "error", err)
			return 1
		}
		return 0
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		logger.Error("serve", "error", err)
		return 1
	}
}

func defaultDataDir() string {
	if dir, err := config.DefaultDataDir(); err == nil {
		return dir
	}
	return ".aicli"
}
