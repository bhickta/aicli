package document

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/systemresources"
)

const topperCopyOCRMaxTokens = 5000

func OCRImages(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	workers int,
	progress func(completed int, total int),
) ([]OCRPage, error) {
	return OCRImagesWithLogger(ctx, vision, model, inputs, prompt, workers, nil, progress)
}

func OCRImagesWithLogger(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	workers int,
	logger *slog.Logger,
	progress func(completed int, total int),
) ([]OCRPage, error) {
	if vision == nil {
		return nil, errors.New("provider is required")
	}
	pages := make([]OCRPage, len(inputs))
	startIndex := 0
	completed := 0
	var completedMu sync.Mutex
	preflightFailures := []pageError{}
	if len(inputs) > 0 {
		page, failure, err := validateVisionModel(ctx, vision, model, inputs[0], prompt)
		if err != nil {
			return nil, err
		}
		pages[0] = page
		if failure != nil {
			preflightFailures = append(preflightFailures, *failure)
		} else {
			reportPageProgress(progress, &completed, &completedMu, len(inputs))
		}
		startIndex = 1
	}
	workers = EffectiveOCRWorkersForVisionProvider(workers, len(inputs), vision)
	workflowStart := time.Now()
	logOCRInfo(logger, "OCR image batch started", "pages", len(inputs), "workers", workers, "provider", providerID(vision), "model", model)
	jobs := make(chan int)
	errCh := make(chan pageError, len(inputs))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go ocrImageWorker(ctx, vision, model, inputs, prompt, pages, jobs, errCh, logger, progress, &completed, &completedMu, &wg)
	}
	for index := startIndex; index < len(inputs); index++ {
		select {
		case <-ctx.Done():
			errCh <- pageError{Index: index, Name: inputs[index].Name, Err: ctx.Err()}
			pages[index] = failedOCRPage(inputs[index], ctx.Err())
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	failures := preflightFailures
	for err := range errCh {
		failures = append(failures, err)
	}
	firstPassFailures := len(failures)
	failures = retryFailedOCRPages(ctx, vision, model, inputs, prompt, pages, failures, logger)
	logOCRInfo(logger, "OCR image batch completed", "pages", len(inputs), "workers", workers, "first_pass_failures", firstPassFailures, "remaining_failures", len(failures), "elapsed_ms", time.Since(workflowStart).Milliseconds())
	if len(failures) == len(inputs) && len(inputs) > 0 {
		return pages, pageErrors(failures)
	}
	return pages, nil
}

func validateVisionModel(ctx context.Context, vision provider.Provider, model string, input ImageInput, prompt string) (OCRPage, *pageError, error) {
	page, err := ocrImage(ctx, vision, model, input, prompt)
	if err == nil {
		return page, nil, nil
	}
	if isVisionUnsupportedError(err) {
		return OCRPage{}, nil, fmt.Errorf("OCR model %q does not support images; select a vision OCR model such as unlimited-ocr in the OCR model field, and use text models only for question split/report: %w", model, err)
	}
	failure := pageError{Index: 0, Name: input.Name, Err: err}
	return failedOCRPage(input, err), &failure, nil
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
	logger *slog.Logger,
	progress func(completed int, total int),
	completed *int,
	completedMu *sync.Mutex,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for index := range jobs {
		start := time.Now()
		page, err := ocrImage(ctx, vision, model, inputs[index], prompt)
		if err != nil {
			errCh <- pageError{Index: index, Name: inputs[index].Name, Err: err}
			pages[index] = failedOCRPage(inputs[index], err)
			logOCRWarn(logger, "OCR page failed", "page", inputs[index].Name, "attempt", "parallel", "elapsed_ms", time.Since(start).Milliseconds(), "error", err)
		} else {
			pages[index] = page
			logOCRInfo(logger, "OCR page completed", "page", inputs[index].Name, "attempt", "parallel", "chars", len(page.Text), "elapsed_ms", time.Since(start).Milliseconds())
		}
		reportPageProgress(progress, completed, completedMu, len(inputs))
	}
}

func retryFailedOCRPages(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	pages []OCRPage,
	failures []pageError,
	logger *slog.Logger,
) []pageError {
	if len(failures) == 0 {
		return nil
	}
	logOCRInfo(logger, "OCR retry pass started", "failed_pages", len(failures))
	remaining := make([]pageError, 0, len(failures))
	for _, failure := range failures {
		if failure.Index < 0 || failure.Index >= len(inputs) || ctx.Err() != nil {
			remaining = append(remaining, failure)
			continue
		}
		start := time.Now()
		page, err := ocrImage(ctx, vision, model, inputs[failure.Index], prompt)
		if err != nil {
			remaining = append(remaining, pageError{Index: failure.Index, Name: failure.Name, Err: err})
			pages[failure.Index] = failedOCRPage(inputs[failure.Index], err)
			logOCRWarn(logger, "OCR page retry failed", "page", failure.Name, "attempt", "retry", "elapsed_ms", time.Since(start).Milliseconds(), "error", err)
			continue
		}
		pages[failure.Index] = page
		logOCRInfo(logger, "OCR page retry completed", "page", failure.Name, "attempt", "retry", "chars", len(page.Text), "elapsed_ms", time.Since(start).Milliseconds())
	}
	logOCRInfo(logger, "OCR retry pass completed", "failed_pages", len(failures), "remaining_failures", len(remaining))
	return remaining
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
		MaxTokens:   topperCopyOCRMaxTokens,
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
	if strings.EqualFold(strings.TrimSpace(res.FinishReason), "length") {
		text = strings.TrimSpace(text + "\n\n[OCR truncated: rerun this page if important]")
	}
	return text, nil
}

func isVisionUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "does not support images") ||
		strings.Contains(text, "vision is not supported") ||
		strings.Contains(text, "image input is not supported")
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
		line = normalizeOCRMarkdownArtifacts(line)
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

func normalizeOCRMarkdownArtifacts(line string) string {
	replacements := []struct {
		old string
		new string
	}{
		{`$\rightarrow$`, "->"},
		{`$\\rightarrow$`, "->"},
		{`\rightarrow`, "->"},
		{`\\rightarrow`, "->"},
		{"\rightarrow", "->"},
	}
	for _, replacement := range replacements {
		line = strings.ReplaceAll(line, replacement.old, replacement.new)
	}
	return line
}

type pageError struct {
	Index int
	Name  string
	Err   error
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

func logOCRInfo(logger *slog.Logger, message string, args ...any) {
	if logger == nil {
		return
	}
	logger.Info(message, args...)
}

func logOCRWarn(logger *slog.Logger, message string, args ...any) {
	if logger == nil {
		return
	}
	logger.Warn(message, args...)
}

func providerID(vision provider.Provider) string {
	if vision == nil {
		return ""
	}
	return vision.ID()
}
