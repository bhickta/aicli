package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bhickta/aicli/internal/provider"
)

func (s *Service) splitQuestions(ctx context.Context, model string, pages []Page, workers int, progress func(completed int, total int)) ([]Question, error) {
	if len(pages) == 0 {
		return nil, nil
	}
	workers = EffectiveQuestionWorkers(workers, len(pages))
	jobs := make(chan Page)
	results := make(chan []Question, len(pages))
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	completed := 0
	var completedMu sync.Mutex

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for page := range jobs {
				start := time.Now()
				questions, err := s.splitPageQuestions(ctx, model, page)
				if err != nil {
					s.logWarn("topper copy question split page failed", "page", page.Number, "name", page.Name, "elapsed_ms", elapsedMS(start), "error", err)
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}
				s.logInfo("topper copy question split page completed", "page", page.Number, "name", page.Name, "questions", len(questions), "elapsed_ms", elapsedMS(start))
				results <- questions
				reportQuestionProgress(progress, &completed, &completedMu, len(pages))
			}
		}()
	}
sendPages:
	for _, page := range pages {
		select {
		case <-ctx.Done():
			break sendPages
		case jobs <- page:
		}
	}
	close(jobs)
	wg.Wait()
	close(results)

	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	detected := []Question{}
	for questions := range results {
		for _, question := range questions {
			if question.Status == "" {
				question.Status = "detected"
			}
			detected = append(detected, question)
		}
	}
	if len(detected) == 0 {
		return pageFallbackQuestions(pages), nil
	}
	merged := mergeQuestionBlocks(detected)
	sortQuestions(merged)
	return merged, nil
}

func mergeQuestionBlocks(questions []Question) []Question {
	sortQuestions(questions)
	merged := []Question{}
	seen := map[string]int{}
	for _, question := range questions {
		key := normalizedQuestionKey(question)
		if isContinuationQuestion(question) && len(merged) > 0 {
			appendQuestionBlock(&merged[len(merged)-1], question)
			continue
		}
		if idx, ok := seen[key]; ok {
			appendQuestionBlock(&merged[idx], question)
			continue
		}
		question.ID = key
		seen[key] = len(merged)
		merged = append(merged, question)
	}
	return merged
}

func normalizedQuestionKey(question Question) string {
	key := normalizeQuestionLabel(question.Label)
	if key == "" {
		key = normalizeQuestionLabel(question.ID)
	}
	if key == "" {
		key = fmt.Sprintf("page-%d", firstInt(question.SourcePages))
	}
	return key
}

func isContinuationQuestion(question Question) bool {
	label := strings.ToLower(strings.TrimSpace(question.Label + " " + question.ID))
	return strings.Contains(label, "continuation")
}

func appendQuestionBlock(dst *Question, src Question) {
	dst.AnswerMarkdown = strings.TrimSpace(dst.AnswerMarkdown + "\n\n" + src.AnswerMarkdown)
	dst.SourcePages = appendUniqueInts(dst.SourcePages, src.SourcePages...)
	if dst.Title == "" {
		dst.Title = src.Title
	}
}

func reportQuestionProgress(progress func(completed int, total int), completed *int, completedMu *sync.Mutex, total int) {
	if progress == nil {
		return
	}
	completedMu.Lock()
	*completed = *completed + 1
	done := *completed
	completedMu.Unlock()
	progress(done, total)
}

func (s *Service) splitPageQuestions(ctx context.Context, model string, page Page) ([]Question, error) {
	if isOCRFailureText(page.Text) {
		return pageFallbackQuestions([]Page{page}), nil
	}
	res, err := s.questionProvider.Chat(ctx, provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: topperCopyQuestionPrompt(page),
			},
		},
		Temperature: 0,
		MaxTokens:   3000,
	})
	if err != nil {
		return nil, err
	}
	questions, err := parseQuestionSplit(res.Content, page.Number)
	if err != nil {
		return pageFallbackQuestions([]Page{page}), nil
	}
	return questions, nil
}

func isOCRFailureText(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "> OCR failed for this page:")
}

type questionSplitPayload struct {
	Questions []questionSplitItem `json:"questions"`
}

type questionSplitItem struct {
	Label          string `json:"label"`
	Question       string `json:"question"`
	Title          string `json:"title"`
	AnswerMarkdown string `json:"answer_markdown"`
	Answer         string `json:"answer"`
	Status         string `json:"status"`
}

func parseQuestionSplit(content string, pageNumber int) ([]Question, error) {
	content, err := extractQuestionSplitJSON(content)
	if err != nil {
		return nil, err
	}
	var payload questionSplitPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
		return nil, err
	}
	questions := make([]Question, 0, len(payload.Questions))
	for i, item := range payload.Questions {
		answer := strings.TrimSpace(item.AnswerMarkdown)
		if answer == "" {
			answer = strings.TrimSpace(item.Answer)
		}
		if answer == "" {
			continue
		}
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = strings.TrimSpace(item.Question)
		}
		if label == "" {
			label = fmt.Sprintf("Page %d block %d", pageNumber, i+1)
		}
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "detected"
		}
		questions = append(questions, Question{
			ID:             normalizeQuestionLabel(label),
			Label:          label,
			Title:          strings.TrimSpace(item.Title),
			AnswerMarkdown: answer,
			SourcePages:    []int{pageNumber},
			Status:         status,
		})
	}
	if len(questions) == 0 {
		return nil, errors.New("question split returned no answer blocks")
	}
	return questions, nil
}

func extractQuestionSplitJSON(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("empty question split response")
	}
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		if len(lines) >= 2 {
			content = strings.Join(lines[1:], "\n")
		}
		content = strings.TrimSpace(strings.TrimSuffix(content, "```"))
	}
	if json.Valid([]byte(content)) {
		return content, nil
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end <= start {
		return "", errors.New("question split response did not contain JSON object")
	}
	candidate := strings.TrimSpace(content[start : end+1])
	if !json.Valid([]byte(candidate)) {
		return "", errors.New("question split response contained invalid JSON")
	}
	return candidate, nil
}

func pageFallbackQuestions(pages []Page) []Question {
	questions := make([]Question, 0, len(pages))
	for _, page := range pages {
		questions = append(questions, Question{
			ID:             fmt.Sprintf("page-%d", page.Number),
			Label:          fmt.Sprintf("Page %d", page.Number),
			AnswerMarkdown: page.Text,
			SourcePages:    []int{page.Number},
			Status:         "needs review",
		})
	}
	return questions
}

func EffectiveQuestionWorkers(workers int, total int) int {
	if total <= 0 {
		return 1
	}
	if workers <= 0 {
		workers = runtime.NumCPU() / 4
	}
	if workers < 1 {
		workers = 1
	}
	if workers > total {
		return total
	}
	return workers
}

func appendUniqueInts(values []int, more ...int) []int {
	seen := map[int]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range more {
		if !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func firstInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func normalizeQuestionLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.Trim(label, ".:;-_ ")
	label = regexp.MustCompile(`\s+`).ReplaceAllString(label, "-")
	return label
}

func sortQuestions(questions []Question) {
	for i := 0; i < len(questions)-1; i++ {
		for j := i + 1; j < len(questions); j++ {
			if firstInt(questions[j].SourcePages) < firstInt(questions[i].SourcePages) {
				questions[i], questions[j] = questions[j], questions[i]
			}
		}
	}
}
