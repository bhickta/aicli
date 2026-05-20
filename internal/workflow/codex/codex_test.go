package codex

import (
	"context"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeProvider struct {
	req provider.ChatRequest
}

func (f *fakeProvider) ID() string { return "fake" }

func (f *fakeProvider) Health(context.Context) error { return nil }

func (f *fakeProvider) ListModels(context.Context) ([]provider.Model, error) { return nil, nil }

func (f *fakeProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	f.req = req
	return provider.ChatResponse{Content: " patch plan "}, nil
}

func (f *fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}

func (f *fakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

func TestRunBuildsCodexRequest(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	res, err := New(fp).Run(context.Background(), Request{
		Model:           "gpt-5.2-codex",
		Task:            "Fix failing tests",
		Context:         "go test ./...",
		ReasoningEffort: "high",
		TextVerbosity:   "low",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Output != "patch plan" {
		t.Fatalf("Output = %q, want patch plan", res.Output)
	}
	if fp.req.Model != "gpt-5.2-codex" || fp.req.ReasoningEffort != "high" || fp.req.TextVerbosity != "low" {
		t.Fatalf("chat request = %#v", fp.req)
	}
	if len(fp.req.Messages) != 2 || fp.req.Messages[1].Content == "" {
		t.Fatalf("messages = %#v", fp.req.Messages)
	}
}

func TestRunRejectsEmptyTask(t *testing.T) {
	t.Parallel()

	if _, err := New(&fakeProvider{}).Run(context.Background(), Request{}); err == nil {
		t.Fatal("Run() error = nil, want empty task error")
	}
}
