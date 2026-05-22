package zettel

import (
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
