package news

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunDedupesNewsItems(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "news.json")
	data := `[{"title":"A Big Story","content":"short"},{"title":"a big story","content":"longer content"},{"title":"Other","content":"x"}]`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := New(nil).Run(context.Background(), Request{Path: path})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Duplicates != 1 || len(res.Items) != 2 {
		t.Fatalf("Response = %#v, want one duplicate and two items", res)
	}
	if len(res.Clusters) == 0 {
		t.Fatal("Clusters is empty")
	}
}

func TestRunExportsXLSX(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "news.json")
	out := filepath.Join(dir, "out.xlsx")
	if err := os.WriteFile(path, []byte(`[{"title":"A","content":"B"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := New(nil).Run(context.Background(), Request{Path: path, OutputPath: out})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.OutputPath != out {
		t.Fatalf("OutputPath = %q, want %q", res.OutputPath, out)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output xlsx missing: %v", err)
	}
}
