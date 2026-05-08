package recall

import (
	"context"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeProvider struct {
	last provider.ChatRequest
}

func (f *fakeProvider) ID() string { return "fake" }
func (f *fakeProvider) Health(context.Context) error {
	return nil
}
func (f *fakeProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (f *fakeProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	f.last = req
	return provider.ChatResponse{Content: "* Explain one\n* Detail two\n* Differentiate three\n* Outline four"}, nil
}
func (f *fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (f *fakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

func TestGenerateRequiresNotes(t *testing.T) {
	t.Parallel()

	svc := New(&fakeProvider{})
	_, err := svc.Generate(context.Background(), Request{})
	if err == nil {
		t.Fatal("Generate() error = nil, want notes required")
	}
}

func TestGenerateBuildsStrictRecallPrompt(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	svc := New(fp)
	res, err := svc.Generate(context.Background(), Request{Model: "model", Notes: "federalism notes"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if res.Triggers == "" {
		t.Fatal("Triggers is empty")
	}
	if fp.last.Model != "model" {
		t.Fatalf("Model = %q, want model", fp.last.Model)
	}
	if len(fp.last.Messages) != 1 || fp.last.Messages[0].Content == "" {
		t.Fatalf("Messages = %#v, want one prompt", fp.last.Messages)
	}
}
