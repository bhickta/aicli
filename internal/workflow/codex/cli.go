package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/config"
)

type CLIRequest struct {
	Model            string `json:"model"`
	Task             string `json:"task"`
	Context          string `json:"context"`
	Workdir          string `json:"workdir"`
	Sandbox          string `json:"sandbox"`
	ApprovalPolicy   string `json:"approval_policy"`
	Profile          string `json:"profile"`
	Search           bool   `json:"search"`
	SkipGitRepoCheck bool   `json:"skip_git_repo_check"`
}

type CLIResponse struct {
	Output  string `json:"output"`
	Command string `json:"command"`
	Workdir string `json:"workdir,omitempty"`
}

type CLIRunner interface {
	CombinedOutputWithInput(ctx context.Context, command string, stdin string, args ...string) ([]byte, error)
}

type CLIService struct {
	tools  config.ToolConfig
	runner CLIRunner
}

func NewCLI(tools config.ToolConfig, runner CLIRunner) *CLIService {
	return &CLIService{tools: tools, runner: runner}
}

func (s *CLIService) Run(ctx context.Context, req CLIRequest) (CLIResponse, error) {
	if s.runner == nil {
		return CLIResponse{}, errors.New("runner is required")
	}
	command := strings.TrimSpace(s.tools.CodexCLI)
	if command == "" {
		return CLIResponse{}, errors.New("codex CLI command is not configured")
	}
	prompt := cliPrompt(req)
	if prompt == "" {
		return CLIResponse{}, errors.New("task is required")
	}

	outputPath, cleanup, err := tempOutputPath()
	if err != nil {
		return CLIResponse{}, err
	}
	defer cleanup()

	args, err := codexExecArgs(req, outputPath)
	if err != nil {
		return CLIResponse{}, err
	}
	raw, runErr := s.runner.CombinedOutputWithInput(ctx, command, prompt, args...)
	output := finalCLIOutput(outputPath, raw)
	res := CLIResponse{
		Output:  output,
		Command: command + " exec",
		Workdir: strings.TrimSpace(req.Workdir),
	}
	if runErr != nil {
		return res, fmt.Errorf("codex CLI failed: %w: %s", runErr, limitedOutput(raw))
	}
	if output == "" {
		return res, errors.New("codex CLI completed without a final response")
	}
	return res, nil
}

func codexExecArgs(req CLIRequest, outputPath string) ([]string, error) {
	sandbox, err := normalizeSandbox(req.Sandbox)
	if err != nil {
		return nil, err
	}
	approval, err := normalizeApproval(req.ApprovalPolicy)
	if err != nil {
		return nil, err
	}
	args := []string{"-a", approval}
	if req.Search {
		args = append(args, "--search")
	}
	args = append(args,
		"exec",
		"--color", "never",
		"--output-last-message", outputPath,
		"--sandbox", sandbox,
	)
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if profile := strings.TrimSpace(req.Profile); profile != "" {
		args = append(args, "--profile", profile)
	}
	if workdir := strings.TrimSpace(req.Workdir); workdir != "" {
		args = append(args, "--cd", filepath.Clean(workdir))
	}
	if req.SkipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	args = append(args, "-")
	return args, nil
}

func cliPrompt(req CLIRequest) string {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return ""
	}
	contextText := strings.TrimSpace(req.Context)
	if contextText == "" {
		return task
	}
	return "Task:\n" + task + "\n\nContext:\n" + contextText
}

func normalizeSandbox(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "":
		return "read-only", nil
	case "read-only", "workspace-write", "danger-full-access":
		return strings.TrimSpace(value), nil
	default:
		return "", fmt.Errorf("unsupported codex sandbox mode %q", value)
	}
}

func normalizeApproval(value string) (string, error) {
	switch strings.TrimSpace(value) {
	case "":
		return "never", nil
	case "never", "on-request", "untrusted", "on-failure":
		return strings.TrimSpace(value), nil
	default:
		return "", fmt.Errorf("unsupported codex approval policy %q", value)
	}
}

func tempOutputPath() (string, func(), error) {
	file, err := os.CreateTemp("", "aicli-codex-last-*.txt")
	if err != nil {
		return "", func() {}, err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", func() {}, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

func finalCLIOutput(outputPath string, raw []byte) string {
	if data, err := os.ReadFile(outputPath); err == nil {
		if text := strings.TrimSpace(string(data)); text != "" {
			return text
		}
	}
	return strings.TrimSpace(string(raw))
}

func limitedOutput(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) <= 2000 {
		return text
	}
	return text[:2000] + "..."
}
