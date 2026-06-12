package geminicli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/textutil"
	"github.com/bhickta/aicli/internal/tool"
)

type Runner interface {
	CombinedOutput(ctx context.Context, command string, args ...string) ([]byte, error)
	CombinedOutputWithInput(ctx context.Context, command string, stdin string, args ...string) ([]byte, error)
}

type Provider struct {
	cfg    config.ProviderConfig
	tools  config.ToolConfig
	runner Runner
}

func New(cfg config.ProviderConfig, tools config.ToolConfig, runner Runner) *Provider {
	return &Provider{cfg: cfg, tools: tools, runner: runner}
}

func (p *Provider) ID() string { return p.cfg.ID }

func (p *Provider) Health(ctx context.Context) error {
	// Simple check if gemini command exists and is executable
	command, err := p.command()
	if err != nil {
		return err
	}
	_, err = p.runner.CombinedOutput(ctx, command, "--version")
	return err
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	// Gemini CLI doesn't have an easy "list models" command that returns machine readable output yet.
	// Returning a set of common models for now.
	return []provider.Model{
		{ID: "gemini-2.0-flash", Name: "Gemini 2.0 Flash"},
		{ID: "gemini-2.0-pro-exp", Name: "Gemini 2.0 Pro Experimental"},
		{ID: "gemini-1.5-pro", Name: "Gemini 1.5 Pro"},
		{ID: "gemini-1.5-flash", Name: "Gemini 1.5 Flash"},
	}, nil
}

func (p *Provider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	command, err := p.command()
	if err != nil {
		return provider.ChatResponse{}, err
	}
	prompt := chatPrompt(req.Messages)
	if prompt == "" {
		return provider.ChatResponse{}, errors.New("message is required")
	}

	args := []string{"-p", prompt}
	if model := textutil.FirstNonBlank(req.Model, p.cfg.Model); model != "" {
		args = append(args, "--model", model)
	}

	raw, runErr := p.runner.CombinedOutput(ctx, command, args...)
	if runErr != nil {
		return provider.ChatResponse{}, fmt.Errorf("gemini CLI chat failed: %w: %s", runErr, tool.LimitedOutput(raw, 2000))
	}

	return provider.ChatResponse{Content: string(raw)}, nil
}

func (p *Provider) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(string) error) error {
	res, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return yield(res.Content)
}

func (p *Provider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("gemini CLI provider does not support vision through this interface yet")
}

func (p *Provider) command() (string, error) {
	if p.runner == nil {
		return "", errors.New("runner is required")
	}
	command := strings.TrimSpace(p.tools.GeminiCLI)
	if command == "" {
		return "", errors.New("gemini CLI command is not configured")
	}
	return command, nil
}

func chatPrompt(messages []provider.Message) string {
	if len(messages) == 0 {
		return ""
	}
	if len(messages) == 1 {
		return strings.TrimSpace(messages[0].Content)
	}
	var out strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(message.Role)
		if role == "" {
			role = "user"
		}
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		out.WriteString(messageRoleLabel(role))
		out.WriteString(":\n")
		out.WriteString(content)
	}
	return strings.TrimSpace(out.String())
}

func messageRoleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "assistant":
		return "Assistant"
	case "system":
		return "System"
	default:
		return "User"
	}
}
