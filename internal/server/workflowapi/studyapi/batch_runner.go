package studyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

const (
	defaultStudyBatchProviderID  = "gemini"
	defaultStudyBatchModel       = "models/gemini-flash-lite-latest"
	defaultStudyBatchParallelism = 2
	maxStudyBatchParallelism     = 5
)

type studyBatchRunOptions struct {
	ProviderID  string
	Model       string
	Parallelism int
	ForceOCR    bool
}

type studyBatchCopyResult struct {
	CopyID       string `json:"copy_id"`
	Status       string `json:"status"`
	Error        string `json:"error,omitempty"`
	CacheHit     bool   `json:"cache_hit"`
	APICalls     int    `json:"api_calls"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalTokens  int    `json:"total_tokens"`
}

type studyBatchRunResult struct {
	Batch   storage.StudyBatchRecord `json:"batch"`
	Results []studyBatchCopyResult   `json:"results"`
}

func (h *Handler) startStudyBatchJob(
	w http.ResponseWriter,
	r *http.Request,
	store studyStore,
	batch storage.StudyBatchRecord,
	items []storage.StudyBatchItemRecord,
	copies []storage.StudyCopyRecord,
	options studyBatchRunOptions,
) {
	job := core.NewJob("study-batch", batch.ID)
	batch.JobID = job.ID
	_ = store.SaveStudyBatch(r.Context(), batch)
	h.runtime.StartWorkflowWithResponse(
		w,
		r,
		job,
		map[string]any{"batch": batch, "items": items},
		func(ctx context.Context, progress core.ProgressFunc) (any, error) {
			return h.runStudyBatch(ctx, progress, store, job.ID, batch, copies, options)
		},
	)
}

func (h *Handler) runStudyBatch(
	ctx context.Context,
	progress core.ProgressFunc,
	store studyStore,
	jobID string,
	batch storage.StudyBatchRecord,
	copies []storage.StudyCopyRecord,
	options studyBatchRunOptions,
) (studyBatchRunResult, error) {
	options = normalizedStudyBatchRunOptions(options)
	vision, ok := h.runtime.ProviderFor(options.ProviderID)
	if !ok {
		return studyBatchRunResult{Batch: batch}, core.ErrProviderNotFound
	}
	workers := minInt(options.Parallelism, len(copies))
	if workers < 1 {
		workers = 1
	}
	progress(core.Units(fmt.Sprintf("analyzing topper PDFs with %d worker(s)", workers), 0, len(copies), "copy"))

	jobs := make(chan storage.StudyCopyRecord)
	results := make(chan studyBatchCopyResult, len(copies))
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for copyRecord := range jobs {
				results <- h.runStudyBatchCopy(ctx, store, jobID, batch, copyRecord, vision, options)
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, copyRecord := range copies {
			select {
			case <-ctx.Done():
				return
			case jobs <- copyRecord:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	return h.collectStudyBatchResults(ctx, progress, store, batch, results)
}

func (h *Handler) runStudyBatchCopy(
	ctx context.Context,
	store studyStore,
	jobID string,
	batch storage.StudyBatchRecord,
	copyRecord storage.StudyCopyRecord,
	vision provider.Provider,
	options studyBatchRunOptions,
) studyBatchCopyResult {
	startedAt := time.Now().UTC()
	if err := saveStudyBatchItem(ctx, store, storage.StudyBatchItemRecord{
		BatchID:   batch.ID,
		CopyID:    copyRecord.ID,
		Stage:     batch.Stage,
		Status:    "running",
		Attempt:   1,
		StartedAt: startedAt,
	}); err != nil {
		return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "failed", Error: err.Error()}
	}
	result, err := h.analyzeStudyBatchCopy(ctx, store, jobID, copyRecord, vision, options)
	finishedAt := time.Now().UTC()
	if err != nil {
		_ = saveStudyBatchItem(ctx, store, storage.StudyBatchItemRecord{
			BatchID:    batch.ID,
			CopyID:     copyRecord.ID,
			Stage:      batch.Stage,
			Status:     "failed",
			Error:      err.Error(),
			ErrorKind:  studyBatchErrorKind(err),
			Attempt:    1,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			DurationMS: int(finishedAt.Sub(startedAt).Milliseconds()),
		})
		return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "failed", Error: err.Error()}
	}
	item := storage.StudyBatchItemRecord{
		BatchID:      batch.ID,
		CopyID:       copyRecord.ID,
		Stage:        batch.Stage,
		Status:       "ready",
		Attempt:      1,
		CacheHit:     result.CacheHit,
		APICalls:     result.APICalls,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		TotalTokens:  result.TotalTokens,
		StartedAt:    startedAt,
		FinishedAt:   finishedAt,
		DurationMS:   int(finishedAt.Sub(startedAt).Milliseconds()),
	}
	_ = saveStudyBatchItem(ctx, store, item)
	return result
}

func (h *Handler) analyzeStudyBatchCopy(
	ctx context.Context,
	store studyStore,
	jobID string,
	copyRecord storage.StudyCopyRecord,
	vision provider.Provider,
	options studyBatchRunOptions,
) (studyBatchCopyResult, error) {
	if strings.TrimSpace(copyRecord.SourcePath) == "" {
		return studyBatchCopyResult{}, fmt.Errorf("copy %s has no source PDF path", copyRecord.ID)
	}
	if !options.ForceOCR {
		if shouldSkipStudyBatchCopy(copyRecord) {
			return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready", CacheHit: true}, nil
		}
		synced, err := h.syncStudyCopyFromMatchingTopper(ctx, store, copyRecord, false)
		if err != nil {
			return studyBatchCopyResult{}, err
		}
		if synced {
			return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready", CacheHit: true}, nil
		}
	}
	result, err := h.runDirectPDFAnalysisWithRetry(ctx, copyRecord, vision, options)
	if err != nil {
		return studyBatchCopyResult{}, err
	}
	topperStore, ok := h.runtime.Store().(studyTopperStore)
	if !ok {
		return studyBatchCopyResult{}, fmt.Errorf("topper review archive is not supported by this store")
	}
	record := studyTopperReviewRecord(result, studyTopperReviewMeta{
		JobID:      jobID,
		SourcePath: copyRecord.SourcePath,
		ProviderID: options.ProviderID,
		Model:      options.Model,
		Status:     "ready",
	})
	if err := topperStore.SaveTopperReview(ctx, record); err != nil {
		return studyBatchCopyResult{}, err
	}
	if err := h.syncStudyTopperReviewArtifact(result); err != nil {
		return studyBatchCopyResult{}, err
	}
	if err := saveStudyFromTopperRecordAsCopy(ctx, store, record, copyRecord.ID, copyRecord); err != nil {
		return studyBatchCopyResult{}, err
	}
	out := studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready", APICalls: 1}
	if result.Usage != nil {
		out.InputTokens = result.Usage.InputTokens
		out.OutputTokens = result.Usage.OutputTokens
		out.TotalTokens = result.Usage.TotalTokens
	}
	return out, nil
}

func (h *Handler) runDirectPDFAnalysisWithRetry(
	ctx context.Context,
	copyRecord storage.StudyCopyRecord,
	vision provider.Provider,
	options studyBatchRunOptions,
) (analyze.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		result, err := analyze.New(
			h.runtime.Settings().Tools,
			tool.ExecRunner{},
			vision,
			analyze.WithArtifactDir(studyBatchArtifactDir(h.runtime.DataDir())),
			analyze.WithLogger(h.runtime.Logger()),
		).RunWithProgress(ctx, analyze.Request{
			Model:        options.Model,
			OCRModel:     options.Model,
			Path:         copyRecord.SourcePath,
			OCRInputMode: analyze.OCRInputModePDFDirect,
			ReviewID:     copyRecord.ID,
			ForceOCR:     options.ForceOCR,
		}, nil)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !isTransientStudyBatchError(err) || attempt == 3 {
			break
		}
		timer := time.NewTimer(time.Duration(attempt*attempt) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return analyze.Response{}, ctx.Err()
		case <-timer.C:
		}
	}
	return analyze.Response{}, lastErr
}

func (h *Handler) collectStudyBatchResults(
	ctx context.Context,
	progress core.ProgressFunc,
	store studyStore,
	batch storage.StudyBatchRecord,
	results <-chan studyBatchCopyResult,
) (studyBatchRunResult, error) {
	out := studyBatchRunResult{Batch: batch, Results: []studyBatchCopyResult{}}
	var firstErr error
	for result := range results {
		out.Results = append(out.Results, result)
		if result.Status == "failed" {
			batch.Failed++
			if firstErr == nil {
				firstErr = errors.New(result.Error)
			}
		} else {
			batch.Completed++
		}
		progress(core.Units("analyzing topper PDFs", batch.Completed+batch.Failed, batch.Total, "copy"))
		if batch.Completed+batch.Failed >= batch.Total {
			batch.Status = "completed"
			if batch.Failed > 0 {
				batch.Status = "failed"
			}
		}
		_ = store.SaveStudyBatch(ctx, batch)
	}
	if err := ctx.Err(); err != nil && firstErr == nil {
		firstErr = err
	}
	if batch.Completed+batch.Failed >= batch.Total {
		batch.FinishedAt = time.Now().UTC()
		if !batch.StartedAt.IsZero() {
			batch.DurationMS = int(batch.FinishedAt.Sub(batch.StartedAt).Milliseconds())
		}
		if batch.Failed > 0 && batch.Completed > 0 {
			batch.Status = "partial_failed"
			firstErr = nil
		}
		_ = store.SaveStudyBatch(ctx, batch)
	}
	out.Batch = batch
	return out, firstErr
}

type studyTopperReviewMeta struct {
	JobID      string
	SourcePath string
	ProviderID string
	Model      string
	Status     string
}

func studyTopperReviewRecord(review analyze.Response, meta studyTopperReviewMeta) storage.TopperReviewRecord {
	data, _ := json.Marshal(review)
	return storage.TopperReviewRecord{
		ID:            review.ReviewID,
		JobID:         meta.JobID,
		PDFName:       review.PDFName,
		SourcePath:    meta.SourcePath,
		ProviderID:    meta.ProviderID,
		Model:         meta.Model,
		PageCount:     len(review.Pages),
		QuestionCount: len(review.Questions),
		UnclearCount:  studyTopperUnclearCount(review),
		Status:        meta.Status,
		ReviewJSON:    string(data),
		SearchText:    studyTopperSearchText(review),
		CreatedAt:     time.Now().UTC(),
	}
}

func studyTopperUnclearCount(review analyze.Response) int {
	total := 0
	for _, page := range review.Pages {
		total += page.UnclearCount
	}
	return total
}

func studyTopperSearchText(review analyze.Response) string {
	var b strings.Builder
	b.WriteString(review.PDFName)
	for _, page := range review.Pages {
		b.WriteString("\n")
		b.WriteString(page.Text)
	}
	for _, question := range review.Questions {
		b.WriteString("\n")
		b.WriteString(question.Label)
		b.WriteString(" ")
		b.WriteString(question.Title)
		b.WriteString("\n")
		b.WriteString(question.AnswerMarkdown)
	}
	b.WriteString("\n")
	b.WriteString(review.Report)
	return strings.ToLower(b.String())
}

func (h *Handler) syncStudyTopperReviewArtifact(review analyze.Response) error {
	if h.runtime.DataDir() == "" || review.ReviewID == "" {
		return nil
	}
	path := filepath.Join(h.runtime.DataDir(), "artifacts", "topper-copy", review.ReviewID, "review.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func saveStudyBatchItem(ctx context.Context, store studyStore, item storage.StudyBatchItemRecord) error {
	return store.SaveStudyBatchItem(ctx, item)
}

func isTransientStudyBatchError(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "429") ||
		strings.Contains(text, "too many requests") ||
		strings.Contains(text, "rate limit") ||
		strings.Contains(text, "500") ||
		strings.Contains(text, "502") ||
		strings.Contains(text, "503") ||
		strings.Contains(text, "504") ||
		strings.Contains(text, "timeout") ||
		strings.Contains(text, "temporarily unavailable")
}

func studyBatchErrorKind(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "cancelled"
	}
	if isTransientStudyBatchError(err) {
		return "transient"
	}
	return "permanent"
}

func shouldSkipStudyBatchCopy(copyRecord storage.StudyCopyRecord) bool {
	return copyRecord.QuestionCount > 0 &&
		strings.EqualFold(copyRecord.Status, "ready") &&
		strings.EqualFold(copyRecord.QuestionStatus, "ready")
}

func normalizedStudyBatchRunOptions(options studyBatchRunOptions) studyBatchRunOptions {
	options.ProviderID = firstString(options.ProviderID, defaultStudyBatchProviderID)
	options.Model = firstString(options.Model, defaultStudyBatchModel)
	if options.Parallelism <= 0 {
		options.Parallelism = defaultStudyBatchParallelism
	}
	if options.Parallelism > maxStudyBatchParallelism {
		options.Parallelism = maxStudyBatchParallelism
	}
	return options
}

func (h *Handler) studyBatchProviderSupportsDirectPDF(providerID string) bool {
	p, ok := h.runtime.ProviderFor(providerID)
	if !ok {
		return false
	}
	_, ok = p.(provider.DocumentProcessor)
	return ok
}

func studyBatchArtifactDir(dataDir string) string {
	if dataDir == "" {
		return ""
	}
	return filepath.Join(dataDir, "artifacts")
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
