package document

import (
	"context"
	"encoding/json"
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

type OCRImagesOptions struct {
	Workers   int
	BatchSize int
	Logger    *slog.Logger
	Progress  func(completed int, total int)
}

func OCRImages(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	workers int,
	progress func(completed int, total int),
) ([]OCRPage, error) {
	return OCRImagesWithOptions(ctx, vision, model, inputs, prompt, OCRImagesOptions{Workers: workers, Progress: progress})
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
	return OCRImagesWithOptions(ctx, vision, model, inputs, prompt, OCRImagesOptions{Workers: workers, Logger: logger, Progress: progress})
}

func OCRImagesWithOptions(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	opts OCRImagesOptions,
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
			reportPageProgress(opts.Progress, &completed, &completedMu, len(inputs))
		}
		startIndex = 1
	}
	workers := EffectiveOCRWorkersForVisionProvider(opts.Workers, len(inputs), vision)
	batchSize := EffectiveOCRBatchSizeForVisionProvider(opts.BatchSize, vision)
	workflowStart := time.Now()
	logOCRInfo(opts.Logger, "OCR image batch started", "pages", len(inputs), "workers", workers, "batch_size", batchSize, "provider", providerID(vision), "model", model)
	jobs := make(chan []int)
	errCh := make(chan pageError, len(inputs))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go ocrImageWorker(ctx, vision, model, inputs, prompt, pages, jobs, errCh, opts.Logger, opts.Progress, &completed, &completedMu, &wg)
	}
	for index := startIndex; index < len(inputs); {
		end := index + batchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		batch := make([]int, 0, end-index)
		for batchIndex := index; batchIndex < end; batchIndex++ {
			batch = append(batch, batchIndex)
		}
		select {
		case <-ctx.Done():
			for _, failedIndex := range batch {
				errCh <- pageError{Index: failedIndex, Name: inputs[failedIndex].Name, Err: ctx.Err()}
				pages[failedIndex] = failedOCRPage(inputs[failedIndex], ctx.Err())
			}
		case jobs <- batch:
		}
		index = end
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	failures := preflightFailures
	for err := range errCh {
		failures = append(failures, err)
	}
	firstPassFailures := len(failures)
	failures = retryFailedOCRPages(ctx, vision, model, inputs, prompt, pages, failures, opts.Logger)
	logOCRInfo(opts.Logger, "OCR image batch completed", "pages", len(inputs), "workers", workers, "batch_size", batchSize, "first_pass_failures", firstPassFailures, "remaining_failures", len(failures), "elapsed_ms", time.Since(workflowStart).Milliseconds())
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

func EffectiveOCRBatchSizeForVisionProvider(batchSize int, vision provider.Provider) int {
	if batchSize > 0 {
		if batchSize > 10 {
			return 10
		}
		return batchSize
	}
	if isLocalVisionProvider(vision) {
		return 1
	}
	if isGeminiVisionProvider(vision) {
		return 5
	}
	return 1
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

func isGeminiVisionProvider(vision provider.Provider) bool {
	if vision == nil {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(vision.ID())), "gemini")
}

func ocrImageWorker(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	pages []OCRPage,
	jobs <-chan []int,
	errCh chan<- pageError,
	logger *slog.Logger,
	progress func(completed int, total int),
	completed *int,
	completedMu *sync.Mutex,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for batch := range jobs {
		start := time.Now()
		batchPages, err := ocrImageBatch(ctx, vision, model, inputs, batch, prompt)
		if err != nil {
			for _, index := range batch {
				errCh <- pageError{Index: index, Name: inputs[index].Name, Err: err}
				pages[index] = failedOCRPage(inputs[index], err)
				reportPageProgress(progress, completed, completedMu, len(inputs))
			}
			logOCRWarn(logger, "OCR page batch failed", "pages", batchNames(inputs, batch), "attempt", "parallel", "elapsed_ms", time.Since(start).Milliseconds(), "error", err)
			continue
		}
		for _, page := range batchPages {
			pages[page.Index] = page.OCRPage
			reportPageProgress(progress, completed, completedMu, len(inputs))
		}
		logOCRInfo(logger, "OCR page batch completed", "pages", batchNames(inputs, batch), "attempt", "parallel", "count", len(batchPages), "elapsed_ms", time.Since(start).Milliseconds())
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

type indexedOCRPage struct {
	Index int
	OCRPage
}

func ocrImageBatch(ctx context.Context, vision provider.Provider, model string, inputs []ImageInput, indexes []int, prompt string) ([]indexedOCRPage, error) {
	if len(indexes) == 0 {
		return nil, nil
	}
	if len(indexes) == 1 {
		page, err := ocrImage(ctx, vision, model, inputs[indexes[0]], prompt)
		if err != nil {
			return nil, err
		}
		return []indexedOCRPage{{Index: indexes[0], OCRPage: page}}, nil
	}
	images := make([]provider.VisionImage, 0, len(indexes))
	names := make([]string, 0, len(indexes))
	for _, index := range indexes {
		data := inputs[index].Data
		if data == nil {
			fileData, err := os.ReadFile(inputs[index].Path)
			if err != nil {
				return nil, err
			}
			data = fileData
		}
		mimeType := inputs[index].MIMEType
		if mimeType == "" {
			mimeType = "image/jpeg"
		}
		images = append(images, provider.VisionImage{
			Name:     inputs[index].Name,
			Image:    data,
			MIMEType: mimeType,
		})
		names = append(names, inputs[index].Name)
	}
	res, err := vision.Vision(ctx, provider.VisionRequest{
		Model:       model,
		Prompt:      batchOCRPrompt(prompt, names),
		Images:      images,
		Temperature: 0,
		MaxTokens:   topperCopyOCRMaxTokens * len(indexes),
	})
	if err != nil {
		return nil, err
	}
	byName, err := cleanBatchOCRResponse(res, names)
	if err != nil {
		return nil, err
	}
	pages := make([]indexedOCRPage, 0, len(indexes))
	for _, index := range indexes {
		text, ok := byName[inputs[index].Name]
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("batch OCR response missing %s", inputs[index].Name)
		}
		pages = append(pages, indexedOCRPage{
			Index: index,
			OCRPage: OCRPage{
				Name: inputs[index].Name,
				Path: inputs[index].Path,
				Text: text,
			},
		})
	}
	return pages, nil
}

func batchOCRPrompt(prompt string, names []string) string {
	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\nYou will receive multiple answer-copy page images in order.\n")
	b.WriteString("Return OCR for every image separately using exactly this marker before each page:\n")
	b.WriteString("<!-- OCR_PAGE page-name -->\n")
	b.WriteString("Use these page names only: ")
	b.WriteString(strings.Join(names, ", "))
	b.WriteString("\nDo not merge pages. Do not omit any page.")
	return b.String()
}

func cleanBatchOCRResponse(res provider.ChatResponse, names []string) (map[string]string, error) {
	text := strings.TrimSpace(res.Content)
	if text == "" {
		return nil, errors.New("OCR response was empty")
	}
	if looksLikeServerErrorPage(text) {
		return nil, errors.New("OCR provider returned an error page instead of text")
	}
	blocks := parseBatchOCRBlocks(text)
	if len(blocks) == 0 {
		jsonBlocks := map[string]string{}
		if err := json.Unmarshal([]byte(text), &jsonBlocks); err == nil && len(jsonBlocks) > 0 {
			blocks = jsonBlocks
		}
	}
	if len(blocks) == 0 {
		return nil, errors.New("batch OCR response had no page markers")
	}
	out := make(map[string]string, len(blocks))
	for _, name := range names {
		raw := strings.TrimSpace(blocks[name])
		if raw == "" {
			continue
		}
		cleaned, err := cleanOCRResponse(provider.ChatResponse{Content: raw, FinishReason: res.FinishReason})
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[name] = cleaned
	}
	return out, nil
}

var batchOCRMarkerPattern = regexp.MustCompile(`(?m)^<!--\s*OCR_PAGE\s+([^>]+?)\s*-->\s*$`)

func parseBatchOCRBlocks(text string) map[string]string {
	matches := batchOCRMarkerPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	blocks := map[string]string{}
	for i, match := range matches {
		name := strings.TrimSpace(text[match[2]:match[3]])
		start := match[1]
		end := len(text)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		if name != "" {
			blocks[name] = strings.TrimSpace(text[start:end])
		}
	}
	return blocks
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

func batchNames(inputs []ImageInput, indexes []int) string {
	names := make([]string, 0, len(indexes))
	for _, index := range indexes {
		if index >= 0 && index < len(inputs) {
			names = append(names, inputs[index].Name)
		}
	}
	return strings.Join(names, ",")
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
