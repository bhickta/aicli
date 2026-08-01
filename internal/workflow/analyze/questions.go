package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bhickta/aicli/internal/provider"
)

type questionSplitResult struct {
	Questions        []Question
	Classifications  []PageClassification
	PrintedQuestions []PrintedQuestion
}

type PageClassification struct {
	PageNumber int
	Kind       string
	Confidence float64
	Reason     string
}

type pageQuestionSplit struct {
	Classification   PageClassification
	PrintedQuestions []PrintedQuestion
	Questions        []Question
}

func (s *Service) splitQuestions(
	ctx context.Context,
	model string,
	pages []Page,
	workers int,
	progress func(completed int, total int),
) (questionSplitResult, error) {
	if len(pages) == 0 {
		return questionSplitResult{
			Questions:        []Question{},
			Classifications:  []PageClassification{},
			PrintedQuestions: []PrintedQuestion{},
		}, nil
	}
	workers = EffectiveQuestionWorkers(workers, len(pages))
	jobs := make(chan Page)
	results := make(chan pageQuestionSplit, len(pages))
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
				pageResult, err := s.splitPageQuestions(ctx, model, page)
				if err != nil {
					s.logWarn("topper copy question split page failed", "page", page.Number, "name", page.Name, "elapsed_ms", elapsedMS(start), "error", err)
					select {
					case errCh <- err:
						cancel()
					default:
					}
					return
				}
				s.logInfo(
					"topper copy question split page completed",
					"page",
					page.Number,
					"name",
					page.Name,
					"page_kind",
					pageResult.Classification.Kind,
					"questions",
					len(pageResult.Questions),
					"elapsed_ms",
					elapsedMS(start),
				)
				results <- pageResult
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
		return questionSplitResult{}, err
	default:
	}
	out := questionSplitResult{
		Questions:        []Question{},
		Classifications:  make([]PageClassification, 0, len(pages)),
		PrintedQuestions: []PrintedQuestion{},
	}
	for pageResult := range results {
		out.Classifications = append(out.Classifications, pageResult.Classification)
		out.PrintedQuestions = append(out.PrintedQuestions, pageResult.PrintedQuestions...)
		for _, question := range pageResult.Questions {
			if question.Status == "" {
				question.Status = "detected"
			}
			out.Questions = append(out.Questions, question)
		}
	}
	if len(out.Questions) == 0 {
		return out, nil
	}
	out.Questions = mergeQuestionBlocks(out.Questions)
	out.Questions = attachPrintedQuestionPrompts(out.Questions, out.PrintedQuestions)
	sortQuestions(out.Questions)
	return out, nil
}

func mergeQuestionBlocks(questions []Question) []Question {
	sortQuestions(questions)
	merged := []Question{}
	seen := map[string]int{}
	for _, question := range questions {
		key := normalizedQuestionKey(question)
		if len(merged) > 0 && (isContinuationQuestion(question) || isLikelyGenericContinuation(merged[len(merged)-1], question)) {
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

func isLikelyGenericContinuation(previous, current Question) bool {
	previousPage := lastInt(previous.SourcePages)
	currentPage := firstInt(current.SourcePages)
	if previousPage <= 0 || currentPage != previousPage+1 {
		return false
	}
	if !hasQuestionSubpart(previous.Label) || hasQuestionSubpart(current.Label) {
		return false
	}
	previousNumber := questionNumber(previous.Label)
	return previousNumber != "" && previousNumber == questionNumber(current.Label)
}

func hasQuestionSubpart(label string) bool {
	return strings.ContainsAny(label, "()")
}

func questionNumber(label string) string {
	var number strings.Builder
	foundDigit := false
	for _, char := range label {
		if char >= '0' && char <= '9' {
			foundDigit = true
			number.WriteRune(char)
			continue
		}
		if foundDigit {
			break
		}
	}
	return number.String()
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

func (s *Service) splitPageQuestions(ctx context.Context, model string, page Page) (pageQuestionSplit, error) {
	if isOCRFailureText(page.Text) {
		return pageQuestionSplit{
			Classification: PageClassification{
				PageNumber: page.Number,
				Kind:       "other",
				Confidence: 1,
				Reason:     "OCR failed",
			},
			PrintedQuestions: []PrintedQuestion{},
			Questions:        []Question{},
		}, nil
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
		return pageQuestionSplit{}, err
	}
	result, err := parsePageQuestionSplit(res.Content, page.Number)
	if err != nil {
		return pageQuestionSplit{
			Classification: PageClassification{
				PageNumber: page.Number,
				Kind:       "unknown",
				Confidence: 0,
				Reason:     "model response could not be parsed",
			},
			PrintedQuestions: []PrintedQuestion{},
			Questions:        pageFallbackQuestions([]Page{page}),
		}, nil
	}
	if len(result.Questions) > 0 && result.Classification.Kind != "answer" {
		result.Classification.Kind = "answer"
		result.Classification.Confidence = min(result.Classification.Confidence, 0.5)
		result.Classification.Reason = strings.TrimSpace(
			result.Classification.Reason + "; candidate answer blocks were extracted",
		)
	}
	if result.Classification.Kind == "answer" && len(result.Questions) == 0 {
		result.Questions = pageFallbackQuestions([]Page{page})
	}
	return result, nil
}

func isOCRFailureText(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "> OCR failed for this page:")
}

type questionSplitPayload struct {
	PageKind             string              `json:"page_kind"`
	PageKindConfidence   float64             `json:"page_kind_confidence"`
	ClassificationReason string              `json:"classification_reason"`
	PrintedQuestions     []PrintedQuestion   `json:"printed_questions"`
	Questions            []questionSplitItem `json:"questions"`
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
	result, err := parsePageQuestionSplit(content, pageNumber)
	if err != nil {
		return nil, err
	}
	if len(result.Questions) == 0 {
		return nil, errors.New("question split returned no answer blocks")
	}
	return result.Questions, nil
}

func parsePageQuestionSplit(content string, pageNumber int) (pageQuestionSplit, error) {
	content, err := extractQuestionSplitJSON(content)
	if err != nil {
		return pageQuestionSplit{}, err
	}
	var payload questionSplitPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(content)), &payload); err != nil {
		return pageQuestionSplit{}, err
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
	kind := normalizePageKind(payload.PageKind)
	if kind == "unknown" && len(questions) > 0 {
		kind = "answer"
	}
	return pageQuestionSplit{
		Classification: PageClassification{
			PageNumber: pageNumber,
			Kind:       kind,
			Confidence: clampFloat(payload.PageKindConfidence, 0, 1),
			Reason:     strings.TrimSpace(payload.ClassificationReason),
		},
		PrintedQuestions: cleanPrintedQuestions(payload.PrintedQuestions),
		Questions:        questions,
	}, nil
}

func normalizePageKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "answer", "question_paper", "cover", "index", "evaluation", "blank", "other":
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return "unknown"
	}
}

func cleanPrintedQuestions(items []PrintedQuestion) []PrintedQuestion {
	cleaned := make([]PrintedQuestion, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item.Label = strings.TrimSpace(item.Label)
		item.Prompt = strings.TrimSpace(item.Prompt)
		key := questionReference(item.Label)
		if key == "" || item.Prompt == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, item)
	}
	return cleaned
}

func clampFloat(value, lower, upper float64) float64 {
	return max(lower, min(value, upper))
}

func limitString(s string, limit int) string {
	if len(s) > limit {
		return s[:limit] + "... [truncated]"
	}
	return s
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
		return "", fmt.Errorf("question split response did not contain JSON object. Raw output: %s", limitString(content, 1000))
	}
	candidate := strings.TrimSpace(content[start : end+1])
	if !json.Valid([]byte(candidate)) {
		return "", fmt.Errorf("question split response contained invalid JSON. Raw output: %s", limitString(candidate, 1000))
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

func lastInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[len(values)-1]
}

func normalizeQuestionLabel(label string) string {
	label = strings.ToLower(strings.TrimSpace(label))
	label = strings.Trim(label, ".:;-_ ")
	label = strings.Join(strings.Fields(label), "-")
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
