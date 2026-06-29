package analyze

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"github.com/bhickta/aicli/internal/provider"
)

func (s *Service) extractDimensions(ctx context.Context, model string, questions []Question, workers int, progress func(completed, total int)) []Question {
	if len(questions) == 0 {
		return questions
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(questions) {
		workers = len(questions)
	}

	results := make([]Question, len(questions))
	copy(results, questions)

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	completed := 0
	var completedMu sync.Mutex

	for i, q := range questions {
		if q.AnswerMarkdown == "" || q.Status == "needs review" {
			completedMu.Lock()
			completed++
			progress(completed, len(questions))
			completedMu.Unlock()
			continue
		}
		wg.Add(1)
		go func(index int, question Question) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			res, err := s.questionProvider.Chat(ctx, provider.ChatRequest{
				Model: model,
				Messages: []provider.Message{
					{
						Role:    "user",
						Content: questionDimensionsPrompt(question),
					},
				},
				Temperature: 0,
				MaxTokens:   2000,
			})

			var dims QuestionDimensions
			if err == nil {
				text := strings.TrimSpace(res.Content)
				// Basic sanitization in case the model returns markdown fences despite instructions
				text = strings.TrimPrefix(text, "```json")
				text = strings.TrimPrefix(text, "```")
				text = strings.TrimSuffix(text, "```")
				text = strings.TrimSpace(text)
				if err := json.Unmarshal([]byte(text), &dims); err != nil {
					s.logWarn("failed to unmarshal dimensions json", "question", question.Label, "error", err, "text", text)
				}
			} else {
				s.logWarn("failed to extract dimensions", "question", question.Label, "error", err)
			}

			// Even if JSON fails or is empty, we attach whatever we successfully parsed
			results[index].Dimensions = &dims

			completedMu.Lock()
			completed++
			progress(completed, len(questions))
			completedMu.Unlock()
		}(i, q)
	}
	wg.Wait()
	return results
}
