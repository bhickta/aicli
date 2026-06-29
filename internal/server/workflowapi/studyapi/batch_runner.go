package studyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	defaultStudyBatchModel       = "gemini-2.5-flash-lite"
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
	CopyID string `json:"copy_id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
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
	if err := saveStudyBatchItemStatus(ctx, store, batch, copyRecord.ID, "running", ""); err != nil {
		return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "failed", Error: err.Error()}
	}
	err := h.analyzeStudyBatchCopy(ctx, store, jobID, copyRecord, vision, options)
	if err != nil {
		_ = saveStudyBatchItemStatus(ctx, store, batch, copyRecord.ID, "failed", err.Error())
		return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "failed", Error: err.Error()}
	}
	_ = saveStudyBatchItemStatus(ctx, store, batch, copyRecord.ID, "ready", "")
	return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready"}
}

func (h *Handler) analyzeStudyBatchCopy(
	ctx context.Context,
	store studyStore,
	jobID string,
	copyRecord storage.StudyCopyRecord,
	vision provider.Provider,
	options studyBatchRunOptions,
) error {
	if strings.TrimSpace(copyRecord.SourcePath) == "" {
		return fmt.Errorf("copy %s has no source PDF path", copyRecord.ID)
	}
	if !options.ForceOCR {
		if shouldSkipStudyBatchCopy(copyRecord) {
			return nil
		}
		synced, err := h.syncStudyCopyFromMatchingTopper(ctx, store, copyRecord, false)
		if err != nil {
			return err
		}
		if synced {
			return nil
		}
	}
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
	if err != nil {
		return err
	}
	topperStore, ok := h.runtime.Store().(studyTopperStore)
	if !ok {
		return fmt.Errorf("topper review archive is not supported by this store")
	}
	record := studyTopperReviewRecord(result, studyTopperReviewMeta{
		JobID:      jobID,
		SourcePath: copyRecord.SourcePath,
		ProviderID: options.ProviderID,
		Model:      options.Model,
		Status:     "ready",
	})
	if err := topperStore.SaveTopperReview(ctx, record); err != nil {
		return err
	}
	if err := h.syncStudyTopperReviewArtifact(result); err != nil {
		return err
	}
	return saveStudyFromTopperRecordAsCopy(ctx, store, record, copyRecord.ID, copyRecord)
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

func saveStudyBatchItemStatus(ctx context.Context, store studyStore, batch storage.StudyBatchRecord, copyID string, status string, itemErr string) error {
	return store.SaveStudyBatchItem(ctx, storage.StudyBatchItemRecord{
		BatchID: batch.ID,
		CopyID:  copyID,
		Stage:   batch.Stage,
		Status:  status,
		Error:   itemErr,
	})
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
