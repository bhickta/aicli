package zettel

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
)

func TestServiceWorkflowOptionsMovesRelativeDataFolderToCentralDataDir(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	vaultDir := t.TempDir()
	service := New(nil).WithDataDir(dataDir)

	options := service.workflowOptions(Options{
		VaultPath:  vaultDir,
		RootFolder: "zettelkasten",
		DataFolder: ".aicli-zettel-merge",
	})

	if !filepath.IsAbs(options.DataFolder) {
		t.Fatalf("data folder = %q, want absolute central path", options.DataFolder)
	}
	if !strings.HasPrefix(options.DataFolder, filepath.Join(dataDir, "zettel")+string(filepath.Separator)) {
		t.Fatalf("data folder = %q, want inside %q", options.DataFolder, dataDir)
	}
}

func TestServiceIndexUsesCentralDataFolder(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "active.md"), "# Active\n")

	embeddingProvider := &fakeZettelProvider{}
	service := NewWithEmbedding(nil, embeddingProvider).WithDataDir(dataDir)
	resp, err := service.Index(context.Background(), IndexRequest{
		Options: Options{
			VaultPath:      vaultDir,
			RootFolder:     "zettelkasten",
			DataFolder:     ".aicli-zettel-merge",
			EmbeddingModel: "text-embedding-nomic-embed-text-v1.5",
		},
	}, nil)
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if resp.Updated != 1 {
		t.Fatalf("Index() = %#v, want one updated note", resp)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".aicli-zettel-merge")); !os.IsNotExist(err) {
		t.Fatalf("vault data folder exists or stat failed unexpectedly: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dataDir, "zettel", "*", "index", "embeddings.json"))
	if err != nil {
		t.Fatalf("glob central embedding cache: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("central embedding caches = %v, want one", matches)
	}
}

func TestServiceTrainingExportUsesCentralDataFolder(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	vaultDir := t.TempDir()
	service := NewWithProviders(nil, nil).WithDataDir(dataDir)
	options := Options{
		VaultPath:  vaultDir,
		RootFolder: "zettelkasten",
		DataFolder: ".aicli-zettel-merge",
	}
	resolved := service.workflowOptions(options)
	trainingArchive := filepath.Join(
		resolved.DataFolder,
		"inbox-runs",
		"run-1",
		"training",
		"zettel-inbox-chat.jsonl",
	)
	writeTrainingArchive(t, trainingArchive)

	resp, err := service.TrainingExport(context.Background(), TrainingExportRequest{Options: options}, nil)
	if err != nil {
		t.Fatalf("TrainingExport() error = %v", err)
	}
	if resp.ExportedCount != 1 || resp.TrainCount != 1 || resp.EvalCount != 0 {
		t.Fatalf("TrainingExport() = %#v, want one train example", resp)
	}
	if !strings.HasPrefix(resp.ArchivePath, filepath.Join(dataDir, "zettel")+string(filepath.Separator)) {
		t.Fatalf("archive path = %q, want inside central data dir %q", resp.ArchivePath, dataDir)
	}
	if _, err := os.Stat(filepath.Join(vaultDir, ".aicli-zettel-merge")); !os.IsNotExist(err) {
		t.Fatalf("vault data folder exists or stat failed unexpectedly: %v", err)
	}
}

func writeTrainingArchive(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	exchange := archivepkg.LLMExchange{
		Workflow:     "zettel-inbox-merge",
		Step:         "merge-final-notes",
		ParsedFormat: "final-notes",
		Request: provider.ChatRequest{
			Messages: []provider.Message{
				{Role: "system", Content: "merge"},
				{Role: "user", Content: "source"},
			},
		},
		Response: provider.ChatResponse{Content: "final"},
	}
	data, err := json.Marshal(exchange)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
}
