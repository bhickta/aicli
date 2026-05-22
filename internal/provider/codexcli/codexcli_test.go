package codexcli

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct {
	command string
	args    []string
	stdin   string
	output  []byte
	err     error
}

func (f *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	f.command = command
	f.args = append([]string{}, args...)
	if f.output != nil || f.err != nil {
		return f.output, f.err
	}
	return []byte(`{"models":[{"slug":"gpt-5.5","display_name":"GPT-5.5"},{"slug":"gpt-5.3-codex","display_name":"GPT-5.3 Codex"}]}`), nil
}

func (f *fakeRunner) CombinedOutputWithInput(_ context.Context, command string, stdin string, args ...string) ([]byte, error) {
	f.command = command
	f.stdin = stdin
	f.args = append([]string{}, args...)
	if f.output != nil || f.err != nil {
		return f.output, f.err
	}
	return []byte("codex chat answer"), nil
}

func TestListModelsUsesCodexDebugModels(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	models, err := New(
		config.ProviderConfig{ID: "codex-cli"},
		config.ToolConfig{CodexCLI: "codex"},
		runner,
	).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if runner.command != "codex" {
		t.Fatalf("command = %q, want codex", runner.command)
	}
	if !slices.Equal(runner.args, []string{"debug", "models"}) {
		t.Fatalf("args = %#v, want debug models", runner.args)
	}
	if len(models) != 2 {
		t.Fatalf("models = %#v, want 2 models", models)
	}
	if models[0] != (provider.Model{ID: "gpt-5.5", Name: "GPT-5.5"}) {
		t.Fatalf("first model = %#v", models[0])
	}
}

func TestListModelsAppliesModelFilter(t *testing.T) {
	t.Parallel()

	models, err := New(
		config.ProviderConfig{ID: "codex-cli", ModelFilter: "codex"},
		config.ToolConfig{CodexCLI: "codex"},
		&fakeRunner{},
	).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-5.3-codex" {
		t.Fatalf("models = %#v, want only codex filtered model", models)
	}
}

func TestChatUsesCodexExec(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	res, err := New(
		config.ProviderConfig{ID: "codex-cli", Model: "gpt-5.5", ReasoningEffort: "medium", TextVerbosity: "low"},
		config.ToolConfig{CodexCLI: "codex"},
		runner,
	).Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "Be exact."},
			{Role: "user", Content: "Reply with ok."},
		},
	})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if res.Content != "codex chat answer" {
		t.Fatalf("Content = %q, want codex chat answer", res.Content)
	}
	for _, want := range []string{"-a", "never", "exec", "--color", "never", "--output-last-message", "--sandbox", "read-only", "--model", "gpt-5.5", "--skip-git-repo-check", "-"} {
		if !slices.Contains(runner.args, want) {
			t.Fatalf("args = %#v, missing %q", runner.args, want)
		}
	}
	for _, want := range []string{`model_reasoning_effort="medium"`, `model_verbosity="low"`} {
		if !slices.Contains(runner.args, want) {
			t.Fatalf("args = %#v, missing config override %q", runner.args, want)
		}
	}
	if !strings.Contains(runner.stdin, "System:\nBe exact.") || !strings.Contains(runner.stdin, "User:\nReply with ok.") {
		t.Fatalf("stdin = %q", runner.stdin)
	}
}

func TestChatReturnsCLIErrorWithOutput(t *testing.T) {
	t.Parallel()

	_, err := New(
		config.ProviderConfig{ID: "codex-cli"},
		config.ToolConfig{CodexCLI: "codex"},
		&fakeRunner{output: []byte("not authenticated"), err: errors.New("exit status 1")},
	).Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("Chat() error = %v, want CLI output included", err)
	}
}
