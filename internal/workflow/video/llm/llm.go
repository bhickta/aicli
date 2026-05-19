package llm

import (
	"context"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type Request struct {
	Model      string `json:"model"`
	Title      string `json:"title"`
	Transcript string `json:"transcript"`
	Mode       string `json:"mode"`
}

type Response struct {
	Text string `json:"text"`
}

type Service struct {
	provider provider.Provider
}

func New(provider provider.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Generate(ctx context.Context, req Request) (Response, error) {
	if s.provider == nil {
		return Response{}, errors.New("provider is required")
	}
	if strings.TrimSpace(req.Transcript) == "" {
		return Response{}, errors.New("transcript is required")
	}
	prompt, err := prompt(req)
	if err != nil {
		return Response{}, err
	}
	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: req.Model,
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   3500,
	})
	if err != nil {
		return Response{}, err
	}
	return Response{Text: strings.TrimSpace(res.Content)}, nil
}

func prompt(req Request) (string, error) {
	title := req.Title
	if title == "" {
		title = "Untitled video"
	}
	switch req.Mode {
	case "", "notes":
		return "Create high-signal study notes for this video transcript. Include headings, bullets, key terms, and action items.\nTitle: " + title + "\n\n" + req.Transcript, nil
	case "tags":
		return "Generate concise searchable tags for this video. Output JSON with keys title, summary, tags, difficulty, topics.\nTitle: " + title + "\n\n" + req.Transcript, nil
	case "course":
		return "Turn this video transcript into a course module plan. Include module title, learning objectives, lesson outline, quiz questions, and prerequisites.\nTitle: " + title + "\n\n" + req.Transcript, nil
	default:
		return "", errors.New("unsupported video LLM mode")
	}
}
