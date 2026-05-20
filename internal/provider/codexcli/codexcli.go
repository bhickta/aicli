package codexcli

import (
	"context"
	"encoding/json"
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
	_, err := p.ListModels(ctx)
	return err
}

func (p *Provider) ListModels(ctx context.Context) ([]provider.Model, error) {
	command, err := p.command()
	if err != nil {
		return nil, err
	}
	raw, err := p.runner.CombinedOutput(ctx, command, "debug", "models")
	if err != nil {
		return nil, fmt.Errorf("list codex cli models: %w: %s", err, tool.LimitedOutput(raw, 2000))
	}
	entries, err := decodeModels(raw)
	if err != nil {
		return nil, err
	}

	models := make([]provider.Model, 0, len(entries))
	for _, entry := range entries {
		id := textutil.FirstNonBlank(entry.Slug, entry.ID, entry.Name, entry.DisplayName)
		if id == "" || !p.allowsModel(id) {
			continue
		}
		name := textutil.FirstNonBlank(entry.DisplayName, entry.Name, id)
		models = append(models, provider.Model{ID: id, Name: name})
	}
	return models, nil
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
	outputPath, cleanup, err := tool.TempOutputPath("aicli-codex-chat-*.txt")
	if err != nil {
		return provider.ChatResponse{}, err
	}
	defer cleanup()

	args := p.chatArgs(req, outputPath)
	raw, runErr := p.runner.CombinedOutputWithInput(ctx, command, prompt, args...)
	output := tool.FinalOutput(outputPath, raw)
	if runErr != nil {
		return provider.ChatResponse{}, fmt.Errorf("codex CLI chat failed: %w: %s", runErr, tool.LimitedOutput(raw, 2000))
	}
	if output == "" {
		return provider.ChatResponse{}, errors.New("codex CLI completed without a final response")
	}
	return provider.ChatResponse{Content: output}, nil
}

func (p *Provider) ChatStream(ctx context.Context, req provider.ChatRequest, yield func(string) error) error {
	res, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return yield(res.Content)
}

func (p *Provider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, errors.New("codex CLI provider does not support vision")
}

func (p *Provider) command() (string, error) {
	if p.runner == nil {
		return "", errors.New("runner is required")
	}
	command := strings.TrimSpace(p.tools.CodexCLI)
	if command == "" {
		return "", errors.New("codex CLI command is not configured")
	}
	return command, nil
}

func (p *Provider) chatArgs(req provider.ChatRequest, outputPath string) []string {
	args := []string{
		"-a", "never",
		"exec",
		"--color", "never",
		"--output-last-message", outputPath,
		"--sandbox", "read-only",
	}
	if model := textutil.FirstNonBlank(req.Model, p.cfg.Model); model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, "--skip-git-repo-check", "-")
	return args
}

func (p *Provider) allowsModel(id string) bool {
	filter := strings.TrimSpace(p.cfg.ModelFilter)
	if filter == "" {
		return true
	}
	id = strings.ToLower(id)
	for _, term := range strings.FieldsFunc(filter, func(r rune) bool {
		return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n'
	}) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && strings.Contains(id, term) {
			return true
		}
	}
	return false
}

type modelEntry struct {
	Slug        string `json:"slug"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func decodeModels(raw []byte) ([]modelEntry, error) {
	var array []modelEntry
	if err := json.Unmarshal(raw, &array); err == nil {
		return array, nil
	}
	var payload struct {
		Models []modelEntry `json:"models"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode codex cli models: %w", err)
	}
	return payload.Models, nil
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
	case "tool":
		return "Tool"
	default:
		return "User"
	}
}
