package analyze

import (
	"context"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func (s *Service) report(ctx context.Context, model string, pages []Page, questions []Question) (string, error) {
	var combined strings.Builder
	if len(questions) > 0 {
		for _, question := range questions {
			combined.WriteString("## ")
			combined.WriteString(question.Label)
			if question.Title != "" {
				combined.WriteString(": ")
				combined.WriteString(question.Title)
			}
			combined.WriteString("\nSource pages: ")
			combined.WriteString(intsString(question.SourcePages))
			combined.WriteString("\n")
			combined.WriteString(question.AnswerMarkdown)
			combined.WriteString("\n\n")
		}
	} else {
		for _, page := range pages {
			combined.WriteString("## Page ")
			combined.WriteString(strconv.Itoa(page.Number))
			combined.WriteString("\n")
			combined.WriteString(page.Text)
			combined.WriteString("\n\n")
		}
	}
	res, err := s.reportProvider.Chat(ctx, provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: topperCopyReportPrompt(combined.String()),
			},
		},
		Temperature: 0,
		MaxTokens:   4000,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Content), nil
}

func intsString(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ", ")
}
