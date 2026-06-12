package gemini

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/tool"
)

type Request struct {
	Model     string `json:"model"`
	Task      string `json:"task"`
	Context   string `json:"context"`
	Workdir   string `json:"workdir"`
	YOLO      bool   `json:"yolo"`
	Sandbox   bool   `json:"sandbox"`
}

type Response struct {
	Output  string `json:"output"`
	Command string `json:"command"`
	Workdir string `json:"workdir,omitempty"`
}

type Runner interface {
	CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error)
}

type Service struct {
	tools  config.ToolConfig
	runner Runner
}

func New(tools config.ToolConfig, runner Runner) *Service {
	return &Service{tools: tools, runner: runner}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if s.runner == nil {
		return Response{}, errors.New("runner is required")
	}
	command := strings.TrimSpace(s.tools.GeminiCLI)
	if command == "" {
		return Response{}, errors.New("gemini CLI command is not configured")
	}
	prompt := geminiPrompt(req)
	if prompt == "" {
		return Response{}, errors.New("task is required")
	}

	args := []string{"-p", prompt}
	if model := strings.TrimSpace(req.Model); model != "" {
		args = append(args, "--model", model)
	}
	if req.YOLO {
		args = append(args, "--yolo")
	}
	if req.Sandbox {
		args = append(args, "--sandbox")
	}
	if workdir := strings.TrimSpace(req.Workdir); workdir != "" {
		// Note: gemini CLI might not have a --cd flag, it might use the current process workdir.
		// For now we just pass it as context if we can't change dir easily.
		// But let's assume it doesn't have it and just use the prompt context.
	}

	raw, runErr := s.runner.CombinedOutput(ctx, command, args...)
	res := Response{
		Output:  string(raw),
		Command: command,
		Workdir: strings.TrimSpace(req.Workdir),
	}
	if runErr != nil {
		return res, fmt.Errorf("gemini CLI failed: %w: %s", runErr, tool.LimitedOutput(raw, 2000))
	}
	return res, nil
}

func geminiPrompt(req Request) string {
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return ""
	}
	var out strings.Builder
	if workdir := strings.TrimSpace(req.Workdir); workdir != "" {
		out.WriteString("Working Directory: ")
		out.WriteString(filepath.Clean(workdir))
		out.WriteString("\n\n")
	}
	if contextText := strings.TrimSpace(req.Context); contextText != "" {
		out.WriteString("Context:\n")
		out.WriteString(contextText)
		out.WriteString("\n\n")
	}
	out.WriteString("Task:\n")
	out.WriteString(task)
	return out.String()
}
