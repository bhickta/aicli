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

const (
	questionBoundaryNew          = "new_answer"
	questionBoundaryContinuation = "continuation"
	questionBoundaryUncertain    = "uncertain"
)

type PageClassification struct {
	PageNumber    int
	Kind          string
	Confidence    float64
	Reason        string
	OCRConfidence *float64
	OCRIssues     []string
}

type pageQuestionSplit struct {
	Classification   PageClassification
	PrintedQuestions []PrintedQuestion
	Questions        []Question
}

type pageQuestionJob struct {
	Page     Page
	Previous *Page
}

type questionSplitRequestError struct {
	err error
}

func (e *questionSplitRequestError) Error() string { return e.err.Error() }
func (e *questionSplitRequestError) Unwrap() error { return e.err }

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
	jobs := make(chan pageQuestionJob)
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
			for job := range jobs {
				start := time.Now()
				pageResult, err := s.splitPageQuestionsWithPrevious(ctx, model, job.Page, job.Previous)
				if err != nil {
					s.logWarn("topper copy question split page failed", "page", job.Page.Number, "name", job.Page.Name, "elapsed_ms", elapsedMS(start), "error", err)
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
					job.Page.Number,
					"name",
					job.Page.Name,
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
	for index, page := range pages {
		var previous *Page
		if index > 0 {
			previousPage := pages[index-1]
			previous = &previousPage
		}
		select {
		case <-ctx.Done():
			break sendPages
		case jobs <- pageQuestionJob{Page: page, Previous: previous}:
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
	usedIDs := map[string]int{}
	for _, question := range questions {
		question.Boundary = normalizeQuestionBoundary(question.Boundary)
		question.BoundaryConfidence = clampFloat(question.BoundaryConfidence, 0, 1)
		if question.Boundary == questionBoundaryContinuation && len(merged) > 0 {
			appendQuestionBlock(&merged[len(merged)-1], question)
			continue
		}
		if question.Boundary == questionBoundaryContinuation {
			question.Boundary = questionBoundaryUncertain
		}
		if question.Boundary == questionBoundaryUncertain || question.BoundaryConfidence < 0.7 {
			question.Status = "needs review"
		}
		question.ID = uniqueQuestionID(question, usedIDs)
		merged = append(merged, question)
	}
	return merged
}

func uniqueQuestionID(question Question, used map[string]int) string {
	base := normalizedQuestionKey(question)
	used[base]++
	if used[base] == 1 {
		return base
	}
	return fmt.Sprintf("%s-%d", base, used[base])
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

func appendQuestionBlock(dst *Question, src Question) {
	dst.AnswerMarkdown = strings.TrimSpace(dst.AnswerMarkdown + "\n\n" + src.AnswerMarkdown)
	dst.SourcePages = appendUniqueInts(dst.SourcePages, src.SourcePages...)
	if dst.Title == "" {
		dst.Title = src.Title
	}
	if src.Status == "needs review" || src.BoundaryConfidence < 0.7 {
		dst.Status = "needs review"
	}
	if src.BoundaryConfidence < dst.BoundaryConfidence {
		dst.BoundaryConfidence = src.BoundaryConfidence
	}
}

func normalizeQuestionBoundary(boundary string) string {
	switch strings.ToLower(strings.TrimSpace(boundary)) {
	case questionBoundaryNew:
		return questionBoundaryNew
	case questionBoundaryContinuation:
		return questionBoundaryContinuation
	default:
		return questionBoundaryUncertain
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
	return s.splitPageQuestionsWithPrevious(ctx, model, page, nil)
}

func (s *Service) splitPageQuestionsWithPrevious(
	ctx context.Context,
	model string,
	page Page,
	previous *Page,
) (pageQuestionSplit, error) {
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
	result, parseErr := s.requestPageQuestionSplit(ctx, model, page, topperCopyQuestionPrompt(page, previous))
	needsRetry := parseErr != nil || (result.Classification.Kind == "question_paper" && len(result.PrintedQuestions) == 0)
	if needsRetry {
		retryResult, retryErr := s.requestPageQuestionSplit(
			ctx,
			model,
			page,
			topperCopyQuestionRetryPrompt(page, previous, pageSplitRetryReason(parseErr, result)),
		)
		if retryErr == nil {
			result = retryResult
			parseErr = nil
		} else if parseErr != nil {
			parseErr = retryErr
		}
	}
	if parseErr != nil {
		var requestErr *questionSplitRequestError
		if errors.As(parseErr, &requestErr) {
			return pageQuestionSplit{}, parseErr
		}
		return pageQuestionSplit{
			Classification: PageClassification{
				PageNumber: page.Number,
				Kind:       "unknown",
				Confidence: 0,
				Reason:     "model response could not be parsed after a focused retry",
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

func (s *Service) requestPageQuestionSplit(
	ctx context.Context,
	model string,
	page Page,
	prompt string,
) (pageQuestionSplit, error) {
	res, err := s.questionProvider.Chat(ctx, provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature:    0,
		MaxTokens:      3000,
		ResponseSchema: pageQuestionSplitJSONSchema(),
	})
	if err != nil {
		return pageQuestionSplit{}, &questionSplitRequestError{err: err}
	}
	return parsePageQuestionSplit(res.Content, page.Number)
}

func pageSplitRetryReason(err error, result pageQuestionSplit) string {
	if err != nil {
		return "The previous response was not valid schema-compliant JSON."
	}
	if result.Classification.Kind == "question_paper" && len(result.PrintedQuestions) == 0 {
		return "The page was identified as a question paper, but its visible printed questions were omitted."
	}
	return "The previous response was incomplete."
}

func isOCRFailureText(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "> OCR failed for this page:")
}

type questionSplitPayload struct {
	PageKind             string              `json:"page_kind"`
	PageKindConfidence   float64             `json:"page_kind_confidence"`
	ClassificationReason string              `json:"classification_reason"`
	OCRConfidence        *float64            `json:"ocr_confidence"`
	OCRIssues            []string            `json:"ocr_issues"`
	PrintedQuestions     []PrintedQuestion   `json:"printed_questions"`
	Questions            []questionSplitItem `json:"questions"`
}

type questionSplitItem struct {
	Boundary           string  `json:"boundary"`
	BoundaryConfidence float64 `json:"boundary_confidence"`
	VisibleLabel       string  `json:"visible_label"`
	Label              string  `json:"label"`
	Question           string  `json:"question"`
	Title              string  `json:"title"`
	AnswerMarkdown     string  `json:"answer_markdown"`
	Answer             string  `json:"answer"`
	Status             string  `json:"status"`
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
		boundary := normalizeQuestionBoundary(item.Boundary)
		boundaryConfidence := clampFloat(item.BoundaryConfidence, 0, 1)
		label := strings.TrimSpace(item.VisibleLabel)
		if label == "" {
			label = strings.TrimSpace(item.Label)
		}
		if label == "" {
			label = strings.TrimSpace(item.Question)
		}
		if label == "" {
			if boundary == questionBoundaryContinuation {
				label = fmt.Sprintf("Page %d continuation", pageNumber)
			} else {
				label = fmt.Sprintf("Page %d block %d", pageNumber, i+1)
			}
		}
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "detected"
		}
		if boundary == questionBoundaryUncertain || boundaryConfidence < 0.7 {
			status = "needs review"
		}
		questions = append(questions, Question{
			ID:                 normalizeQuestionLabel(label),
			Label:              label,
			Title:              strings.TrimSpace(item.Title),
			AnswerMarkdown:     answer,
			SourcePages:        []int{pageNumber},
			Status:             status,
			Boundary:           boundary,
			BoundaryConfidence: boundaryConfidence,
		})
	}
	kind := normalizePageKind(payload.PageKind)
	if kind == "unknown" && len(questions) > 0 {
		kind = "answer"
	}
	return pageQuestionSplit{
		Classification: PageClassification{
			PageNumber:    pageNumber,
			Kind:          kind,
			Confidence:    clampFloat(payload.PageKindConfidence, 0, 1),
			Reason:        strings.TrimSpace(payload.ClassificationReason),
			OCRConfidence: clampedOptionalConfidence(payload.OCRConfidence),
			OCRIssues:     cleanStringList(payload.OCRIssues),
		},
		PrintedQuestions: cleanPrintedQuestions(payload.PrintedQuestions),
		Questions:        questions,
	}, nil
}

func clampedOptionalConfidence(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clamped := clampFloat(*value, 0, 1)
	return &clamped
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
			ID:                 fmt.Sprintf("page-%d", page.Number),
			Label:              fmt.Sprintf("Page %d", page.Number),
			AnswerMarkdown:     page.Text,
			SourcePages:        []int{page.Number},
			Status:             "needs review",
			Boundary:           questionBoundaryUncertain,
			BoundaryConfidence: 0,
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
