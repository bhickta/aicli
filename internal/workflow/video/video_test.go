package video

import (
	"context"

	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct {
	command string
	args    []string
	out     []byte
	err     error
	calls   []runnerCall
}

type runnerCall struct {
	command string
	args    []string
}

func (f *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	f.command = command
	f.args = args
	f.calls = append(f.calls, runnerCall{command: command, args: append([]string(nil), args...)})
	return f.out, f.err
}

type fakeProvider struct{}

func (fakeProvider) ID() string { return "fake" }
func (fakeProvider) Health(context.Context) error {
	return nil
}
func (fakeProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (fakeProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: "notes"}, nil
}
func (fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (fakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}
