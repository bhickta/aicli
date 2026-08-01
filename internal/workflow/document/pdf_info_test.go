package document

import (
	"context"
	"reflect"
	"testing"
)

type commandCaptureRunner struct {
	command string
	args    []string
	out     []byte
}

func (r *commandCaptureRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	r.command = command
	r.args = append([]string(nil), args...)
	return r.out, nil
}

func TestPDFPageCount(t *testing.T) {
	t.Parallel()

	runner := &commandCaptureRunner{out: []byte("Title: copy\nPages: 52\n")}
	pages, err := PDFPageCount(context.Background(), runner, "/opt/poppler/pdftoppm", "copy.pdf")
	if err != nil {
		t.Fatalf("PDFPageCount() error = %v", err)
	}
	if pages != 52 {
		t.Fatalf("pages = %d, want 52", pages)
	}
	if runner.command != "/opt/poppler/pdfinfo" {
		t.Fatalf("command = %q, want /opt/poppler/pdfinfo", runner.command)
	}
}

func TestSplitPDFRange(t *testing.T) {
	t.Parallel()

	runner := &commandCaptureRunner{}
	err := SplitPDFRange(context.Background(), runner, "custom-qpdf", "copy.pdf", "chunk.pdf", 7, 14)
	if err != nil {
		t.Fatalf("SplitPDFRange() error = %v", err)
	}
	if runner.command != "custom-qpdf" {
		t.Fatalf("command = %q, want custom-qpdf", runner.command)
	}
	want := []string{"copy.pdf", "--pages", ".", "7-14", "--", "chunk.pdf"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("args = %#v, want %#v", runner.args, want)
	}
}

func TestSplitPDFRangeRejectsInvalidRange(t *testing.T) {
	t.Parallel()

	runner := &commandCaptureRunner{}
	if err := SplitPDFRange(context.Background(), runner, "qpdf", "copy.pdf", "chunk.pdf", 8, 7); err == nil {
		t.Fatal("SplitPDFRange() error = nil, want invalid range error")
	}
	if runner.command != "" {
		t.Fatalf("command = %q, want no command", runner.command)
	}
}
