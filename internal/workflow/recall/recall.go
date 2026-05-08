package recall

import (
	"context"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type Service struct {
	provider provider.Provider
}

type Request struct {
	Model string `json:"model"`
	Notes string `json:"notes"`
}

type Response struct {
	Triggers string `json:"triggers"`
}

func New(provider provider.Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Generate(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Notes) == "" {
		return Response{}, errors.New("notes are required")
	}

	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: req.Model,
		Messages: []provider.Message{
			{Role: "user", Content: prompt(req.Notes)},
		},
		Temperature: 0,
	})
	if err != nil {
		return Response{}, err
	}
	return Response{Triggers: strings.TrimSpace(res.Content)}, nil
}

func prompt(notes string) string {
	return `<INSTRUCTION>
You are a strict UPSC Examiner. Generate exactly 4 broad recall triggers based on the NOTES provided.

STRICT RULES:
1. CONCEPT PARTITIONING: Each trigger must test a different concept from the notes.
2. Format: Output a simple list starting with * bullet points. No introductory or concluding text.
3. Verb: Start every bullet with Explain, Detail, Differentiate, or Outline.
4. Style: Keep triggers broad. Do not include specific names, numbers, or definitions in the triggers themselves.
</INSTRUCTION>

<NOTES>
` + notes + `
</NOTES>`
}
