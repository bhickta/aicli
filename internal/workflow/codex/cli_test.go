package codex

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

type fakeCLIRunner struct {
	command string
	stdin   string
	args    []string
	out     []byte
}

func (f *fakeCLIRunner) CombinedOutputWithInput(_ context.Context, command string, stdin string, args ...string) ([]byte, error) {
	f.command = command
	f.stdin = stdin
	f.args = append([]string{}, args...)
	if f.out != nil {
		return f.out, nil
	}
	return []byte("codex cli answer"), nil
}

func TestCLIRunBuildsCodexExecCommand(t *testing.T) {
	t.Parallel()

	runner := &fakeCLIRunner{}
	res, err := NewCLI(config.ToolConfig{CodexCLI: "codex"}, runner).Run(context.Background(), CLIRequest{
		Model:            "gpt-5.5",
		Task:             "Fix tests",
		Context:          "go test ./...",
		Workdir:          "/tmp/project",
		Sandbox:          "workspace-write",
		ApprovalPolicy:   "never",
		Profile:          "pro",
		ReasoningEffort:  "low",
		TextVerbosity:    "low",
		Search:           true,
		SkipGitRepoCheck: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Output != "codex cli answer" || res.Command != "codex exec" {
		t.Fatalf("response = %#v", res)
	}
	if runner.command != "codex" {
		t.Fatalf("command = %q, want codex", runner.command)
	}
	for _, want := range []string{"-a", "never", "exec", "--color", "never", "--output-last-message", "--sandbox", "workspace-write", "--model", "gpt-5.5", "--profile", "pro", "--cd", "/tmp/project", "--skip-git-repo-check", "--search", "-"} {
		if !slices.Contains(runner.args, want) {
			t.Fatalf("args = %#v, missing %q", runner.args, want)
		}
	}
	for _, want := range []string{`model_reasoning_effort="low"`, `model_verbosity="low"`} {
		if !slices.Contains(runner.args, want) {
			t.Fatalf("args = %#v, missing config override %q", runner.args, want)
		}
	}
	if slices.Index(runner.args, "--search") > slices.Index(runner.args, "exec") {
		t.Fatalf("args = %#v, want --search before exec", runner.args)
	}
	if !strings.Contains(runner.stdin, "Fix tests") || !strings.Contains(runner.stdin, "go test ./...") {
		t.Fatalf("stdin = %q", runner.stdin)
	}
}

func TestCLIRunRejectsUnsafeEnumValues(t *testing.T) {
	t.Parallel()

	_, err := NewCLI(config.ToolConfig{CodexCLI: "codex"}, &fakeCLIRunner{}).Run(context.Background(), CLIRequest{
		Task:    "do work",
		Sandbox: "bad-mode",
	})
	if err == nil {
		t.Fatal("Run() error = nil, want invalid sandbox error")
	}
}

func TestCLIRunRejectsMissingCommand(t *testing.T) {
	t.Parallel()

	if _, err := NewCLI(config.ToolConfig{}, &fakeCLIRunner{}).Run(context.Background(), CLIRequest{Task: "do work"}); err == nil {
		t.Fatal("Run() error = nil, want missing command error")
	}
}
