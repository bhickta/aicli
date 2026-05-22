package zettel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
