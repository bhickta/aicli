package zettel

import (
	"path/filepath"
	"testing"
)

func TestListNotesReturnsScopedMarkdownNotes(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "a.md"), "a\n")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", "nested", "b.md"), "b\n")
	writeTestFile(t, filepath.Join(vaultDir, "zettelkasten", ".private", "hidden.md"), "hidden\n")
	writeTestFile(t, filepath.Join(vaultDir, "other", "outside.md"), "outside\n")

	resp, err := ListNotes(Options{
		VaultPath:  vaultDir,
		RootFolder: "zettelkasten",
		DataFolder: ".aicli-zettel-merge",
	})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	want := []string{"zettelkasten/a.md", "zettelkasten/nested/b.md"}
	if resp.Count != len(want) {
		t.Fatalf("count = %d, want %d", resp.Count, len(want))
	}
	for i, note := range want {
		if resp.Notes[i] != note {
			t.Fatalf("notes[%d] = %q, want %q", i, resp.Notes[i], note)
		}
	}
}
