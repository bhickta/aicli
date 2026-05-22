package codex

import (
	"context"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type Request struct {
	Model           string `json:"model"`
	Task            string `json:"task"`
	Context         string `json:"context"`
	ReasoningEffort string `json:"reasoning_effort"`
	TextVerbosity   string `json:"text_verbosity"`
}

type Response struct {
	Output string               `json:"output"`
	Usage  *provider.TokenUsage `json:"usage,omitempty"`
}

type Service struct {
	provider provider.Provider
}

func New(provider provider.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if s.provider == nil {
		return Response{}, errors.New("provider is required")
	}
	task := strings.TrimSpace(req.Task)
	if task == "" {
		return Response{}, errors.New("task is required")
	}

	userPrompt := task
	if contextText := strings.TrimSpace(req.Context); contextText != "" {
		userPrompt = "Task:\n" + task + "\n\nContext:\n" + contextText
	}
	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model:           req.Model,
		Messages:        codexMessages(userPrompt),
		Temperature:     0.1,
		ReasoningEffort: req.ReasoningEffort,
		TextVerbosity:   req.TextVerbosity,
	})
	if err != nil {
		return Response{}, err
	}
	return Response{Output: strings.TrimSpace(res.Content), Usage: res.Usage}, nil
}

func codexMessages(userPrompt string) []provider.Message {
	return []provider.Message{
		{
			Role:    "system",
			Content: "You are Codex, a pragmatic coding assistant. Answer with concrete implementation guidance, code review findings, or patch-ready steps. Be concise and preserve technical constraints from the user.",
		},
		{
			Role:    "user",
			Content: userPrompt,
		},
	}
}
