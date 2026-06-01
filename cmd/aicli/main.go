package main

import (
	"context"
	"encoding/json"
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
	zettelmodel "github.com/bhickta/aicli/internal/workflow/zettel/model"
	zetteltraining "github.com/bhickta/aicli/internal/workflow/zettel/training"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 && args[0] == "zettel-training-export" {
		return runZettelTrainingExport(args[1:])
	}

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

func runZettelTrainingExport(args []string) int {
	var vaultPath string
	var dataFolder string
	var strict bool
	var jsonOutput bool

	fs := flag.NewFlagSet("aicli zettel-training-export", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&vaultPath, "vault", "", "Obsidian vault path")
	fs.StringVar(&dataFolder, "data-folder", ".aicli-zettel-merge", "zettel data folder, relative to vault or absolute")
	fs.BoolVar(&strict, "strict", true, "exclude prompt/output noise before exporting")
	fs.BoolVar(&jsonOutput, "json", false, "print full export manifest JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if vaultPath == "" {
		fmt.Fprintln(os.Stderr, "missing -vault")
		return 2
	}

	response, err := zetteltraining.New().Export(context.Background(), zetteltraining.TrainingExportRequest{
		Options: zettelmodel.Options{
			VaultPath:  vaultPath,
			DataFolder: dataFolder,
		},
		Strict: strict,
	}, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "training export failed: %v\n", err)
		return 1
	}
	if jsonOutput {
		data, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal response: %v\n", err)
			return 1
		}
		fmt.Fprintln(os.Stdout, string(data))
		return 0
	}

	fmt.Fprintf(os.Stdout, "Run: %s\n", response.RunID)
	fmt.Fprintf(os.Stdout, "Train: %s (%d)\n", response.TrainPath, response.TrainCount)
	fmt.Fprintf(os.Stdout, "Eval: %s (%d)\n", response.EvalPath, response.EvalCount)
	fmt.Fprintf(os.Stdout, "ShareGPT train: %s\n", response.ShareGPTTrainPath)
	fmt.Fprintf(os.Stdout, "ShareGPT eval: %s\n", response.ShareGPTEvalPath)
	fmt.Fprintf(os.Stdout, "Manifest: %s\n", response.ManifestPath)
	fmt.Fprintf(os.Stdout, "Skipped: %d\n", response.SkippedCount)
	return 0
}

func defaultDataDir() string {
	if dir, err := config.DefaultDataDir(); err == nil {
		return dir
	}
	return ".aicli"
}
