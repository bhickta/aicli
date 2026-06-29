package document

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/systemresources"
)

func OCRImages(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	workers int,
	progress func(completed int, total int),
) ([]OCRPage, error) {
	if vision == nil {
		return nil, errors.New("provider is required")
	}
	workers = EffectiveOCRWorkersForVisionProvider(workers, len(inputs), vision)
	pages := make([]OCRPage, len(inputs))
	jobs := make(chan int)
	errCh := make(chan pageError, len(inputs))
	completed := 0
	var completedMu sync.Mutex

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go ocrImageWorker(ctx, vision, model, inputs, prompt, pages, jobs, errCh, progress, &completed, &completedMu, &wg)
	}
	for index := range inputs {
		select {
		case <-ctx.Done():
			errCh <- pageError{Name: inputs[index].Name, Err: ctx.Err()}
			pages[index] = failedOCRPage(inputs[index], ctx.Err())
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	failures := []pageError{}
	for err := range errCh {
		failures = append(failures, err)
	}
	if len(failures) == len(inputs) && len(inputs) > 0 {
		return pages, pageErrors(failures)
	}
	return pages, nil
}

func EffectiveOCRWorkers(workers int, jobs int) int {
	return effectiveOCRWorkers(workers, jobs)
}

func EffectiveOCRWorkersForProvider(workers int, jobs int, providerID string) int {
	requestedWorkers := workers
	workers = effectiveOCRWorkers(workers, jobs)
	if requestedWorkers <= 0 {
		if limit := localOCRWorkerLimit(providerID); limit > 0 && workers > limit {
			return limit
		}
	}
	return workers
}

func EffectiveOCRWorkersForVisionProvider(workers int, jobs int, vision provider.Provider) int {
	requestedWorkers := workers
	workers = effectiveOCRWorkers(workers, jobs)
	if requestedWorkers <= 0 && isLocalVisionProvider(vision) && workers > 1 {
		return 1
	}
	return workers
}

func effectiveOCRWorkers(workers int, jobs int) int {
	if jobs <= 1 {
		return 1
	}
	if workers < 1 {
		return systemresources.DefaultOCRWorkers(jobs, systemresources.Snapshot{})
	}
	if workers > jobs {
		return jobs
	}
	return workers
}

func localOCRWorkerLimit(providerID string) int {
	id := strings.ToLower(strings.TrimSpace(providerID))
	switch id {
	case "lms", "lmstudio", "lm-studio", "lm_studio", "ollama", "vllm":
		return 1
	default:
		return 0
	}
}

func isLocalVisionProvider(vision provider.Provider) bool {
	if vision == nil {
		return false
	}
	if local, ok := vision.(provider.LocalModelServer); ok && local.LocalModelServer() {
		return true
	}
	return localOCRWorkerLimit(vision.ID()) > 0
}

func ocrImageWorker(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	pages []OCRPage,
	jobs <-chan int,
	errCh chan<- pageError,
	progress func(completed int, total int),
	completed *int,
	completedMu *sync.Mutex,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for index := range jobs {
		page, err := ocrImage(ctx, vision, model, inputs[index], prompt)
		if err != nil {
			errCh <- pageError{Name: inputs[index].Name, Err: err}
			pages[index] = failedOCRPage(inputs[index], err)
		} else {
			pages[index] = page
		}
		reportPageProgress(progress, completed, completedMu, len(inputs))
	}
}

func reportPageProgress(progress func(completed int, total int), completed *int, completedMu *sync.Mutex, total int) {
	if progress == nil {
		return
	}
	completedMu.Lock()
	*completed = *completed + 1
	done := *completed
	completedMu.Unlock()
	progress(done, total)
}

func ocrImage(ctx context.Context, vision provider.Provider, model string, input ImageInput, prompt string) (OCRPage, error) {
	data := input.Data
	if data == nil {
		fileData, err := os.ReadFile(input.Path)
		if err != nil {
			return OCRPage{}, err
		}
		data = fileData
	}
	mimeType := input.MIMEType
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	res, err := vision.Vision(ctx, provider.VisionRequest{
		Model:       model,
		Prompt:      prompt,
		Image:       data,
		MIMEType:    mimeType,
		Temperature: 0,
		MaxTokens:   2200,
	})
	if err != nil {
		return OCRPage{}, err
	}
	text, err := cleanOCRResponse(res)
	if err != nil {
		return OCRPage{}, err
	}
	return OCRPage{
		Name: input.Name,
		Path: input.Path,
		Text: text,
	}, nil
}

func cleanOCRResponse(res provider.ChatResponse) (string, error) {
	if strings.EqualFold(strings.TrimSpace(res.FinishReason), "length") {
		return "", errors.New("OCR response was truncated by the model token limit")
	}
	text := strings.TrimSpace(res.Content)
	if text == "" {
		return "", errors.New("OCR response was empty")
	}
	if looksLikeServerErrorPage(text) {
		return "", errors.New("OCR provider returned an error page instead of text")
	}
	text = stripDetectionTags(text)
	text = collapseDuplicateOCRLines(text)
	if strings.TrimSpace(text) == "" {
		return "", errors.New("OCR response had no readable text")
	}
	return text, nil
}

func looksLikeServerErrorPage(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "<html") ||
		strings.Contains(lower, "</body>") ||
		strings.Contains(lower, "<pre>internal server error</pre>") ||
		strings.Contains(lower, "internal server error")
}

var detectionTagPattern = regexp.MustCompile(`<\|/?det\|>`)
var detectionBoxPrefixPattern = regexp.MustCompile(`^(?:[A-Za-z_-]+\s+)?\[\d+,\s*\d+,\s*\d+,\s*\d+\]\s*`)

func stripDetectionTags(text string) string {
	text = detectionTagPattern.ReplaceAllString(text, "")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(detectionBoxPrefixPattern.ReplaceAllString(line, ""))
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func collapseDuplicateOCRLines(text string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	seenLong := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(line), " "))
		if len([]rune(key)) >= 20 {
			if seenLong[key] {
				continue
			}
			seenLong[key] = true
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

type pageError struct {
	Name string
	Err  error
}

type pageErrors []pageError

func (e pageErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, item := range e {
		name := item.Name
		if name == "" {
			name = "page"
		}
		parts = append(parts, name+": "+item.Err.Error())
	}
	return strings.Join(parts, "; ")
}

func failedOCRPage(input ImageInput, err error) OCRPage {
	return OCRPage{
		Name: input.Name,
		Path: input.Path,
		Text: "> OCR failed for this page: " + err.Error(),
	}
}
