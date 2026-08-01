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
	defaultStudyBatchProviderID  = "lms"
	defaultStudyBatchParallelism = 1
	maxStudyBatchParallelism     = 5
	defaultStudyBatchDPI         = 220
)

type studyBatchRunOptions struct {
	ProviderID    string
	Model         string
	OCRModel      string
	QuestionModel string
	BoundaryModel string
	ReportModel   string
	Parallelism   int
	ModelWorkers  int
	ForceOCR      bool
}

var errStudyOCRPhaseComplete = errors.New("study OCR phase complete")

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
	var err error
	options, vision, err := h.resolveStudyBatchRunOptions(ctx, options)
	if err != nil {
		return studyBatchRunResult{Batch: batch}, err
	}
	copyWorkers, modelWorkers := studyBatchWorkerAllocation(options.Parallelism, len(copies))
	options.ModelWorkers = modelWorkers
	progress(core.Units(
		fmt.Sprintf(
			"%s with %d copy worker(s), %d model worker(s) per copy",
			studyBatchProgressLabel(batch.Stage),
			copyWorkers,
			modelWorkers,
		),
		0,
		len(copies),
		"copy",
	))

	jobs := make(chan storage.StudyCopyRecord)
	results := make(chan studyBatchCopyResult, len(copies))
	var wg sync.WaitGroup
	for range copyWorkers {
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
	result, err := h.analyzeStudyBatchCopy(ctx, store, jobID, batch.Stage, copyRecord, vision, options)
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
		Status:       firstString(result.Status, "ready"),
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
	stage string,
	copyRecord storage.StudyCopyRecord,
	vision provider.Provider,
	options studyBatchRunOptions,
) (studyBatchCopyResult, error) {
	if stage == "metadata" {
		return h.generateStudyBatchMetadata(ctx, store, copyRecord, vision, options)
	}
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
	topperStore, ok := h.runtime.Store().(studyTopperStore)
	if !ok {
		return studyBatchCopyResult{}, fmt.Errorf("topper review archive is not supported by this store")
	}
	ocrPages, err := loadStudyOCRCheckpoint(ctx, topperStore, copyRecord, options.ForceOCR)
	if err != nil {
		return studyBatchCopyResult{}, err
	}
	if stage == "ocr" {
		if len(ocrPages) > 0 {
			return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready", CacheHit: true}, nil
		}
		if _, err := h.runStudyOCRPhase(ctx, topperStore, jobID, copyRecord, vision, options); err != nil {
			return studyBatchCopyResult{}, err
		}
		return studyBatchCopyResult{CopyID: copyRecord.ID, Status: "ready", APICalls: 1}, nil
	}
	result, err := h.runImageAnalysisWithRetry(ctx, topperStore, jobID, copyRecord, vision, options, ocrPages)
	if err != nil {
		return studyBatchCopyResult{}, err
	}
	record := studyTopperReviewRecord(result, studyTopperReviewMeta{
		JobID:      jobID,
		SourcePath: copyRecord.SourcePath,
		ProviderID: options.ProviderID,
		Model:      options.Model,
		Status:     studyTopperReviewStatus(result),
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
	out := studyBatchCopyResult{CopyID: copyRecord.ID, Status: record.Status, APICalls: firstInt(result.APICalls, 1)}
	if result.Usage != nil {
		out.InputTokens = result.Usage.InputTokens
		out.OutputTokens = result.Usage.OutputTokens
		out.TotalTokens = result.Usage.TotalTokens
	}
	return out, nil
}

func studyTopperReviewStatus(review analyze.Response) string {
	if review.Quality != nil && review.Quality.RequiresReview {
		return "needs_review"
	}
	return "ready"
}

func (h *Handler) runImageAnalysisWithRetry(
	ctx context.Context,
	topperStore studyTopperStore,
	jobID string,
	copyRecord storage.StudyCopyRecord,
	vision provider.Provider,
	options studyBatchRunOptions,
	ocrPages []analyze.Page,
) (analyze.Response, error) {
	if len(ocrPages) == 0 && !sameStudyModel(options.OCRModel, options.QuestionModel) {
		var err error
		ocrPages, err = h.runStudyOCRPhase(ctx, topperStore, jobID, copyRecord, vision, options)
		if err != nil {
			return analyze.Response{}, err
		}
	}
	if len(ocrPages) > 0 && !sameStudyModel(options.OCRModel, options.QuestionModel) {
		if unloader, ok := vision.(provider.ModelUnloader); ok {
			if err := unloader.UnloadModel(ctx, options.OCRModel); err != nil {
				return analyze.Response{}, fmt.Errorf("unload OCR model %q before analysis: %w", options.OCRModel, err)
			}
		}
	}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		service := analyze.New(
			h.runtime.Settings().Tools,
			tool.ExecRunner{},
			vision,
			analyze.WithArtifactDir(studyBatchArtifactDir(h.runtime.DataDir())),
			analyze.WithLogger(h.runtime.Logger()),
			analyze.WithOCRCheckpoint(func(checkpoint analyze.Response) error {
				ocrPages = append([]analyze.Page(nil), checkpoint.Pages...)
				return h.saveStudyOCRCheckpoint(ctx, topperStore, jobID, copyRecord, options, checkpoint)
			}),
		)
		result, err := service.RunWithProgress(ctx, newStudyAnalysisRequest(copyRecord, options, ocrPages), nil)
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

func (h *Handler) runStudyOCRPhase(
	ctx context.Context,
	topperStore studyTopperStore,
	jobID string,
	copyRecord storage.StudyCopyRecord,
	vision provider.Provider,
	options studyBatchRunOptions,
) ([]analyze.Page, error) {
	var pages []analyze.Page
	service := analyze.New(
		h.runtime.Settings().Tools,
		tool.ExecRunner{},
		vision,
		analyze.WithArtifactDir(studyBatchArtifactDir(h.runtime.DataDir())),
		analyze.WithLogger(h.runtime.Logger()),
		analyze.WithOCRCheckpoint(func(checkpoint analyze.Response) error {
			if err := h.saveStudyOCRCheckpoint(ctx, topperStore, jobID, copyRecord, options, checkpoint); err != nil {
				return err
			}
			pages = append([]analyze.Page(nil), checkpoint.Pages...)
			return errStudyOCRPhaseComplete
		}),
	)
	_, err := service.RunWithProgress(ctx, newStudyOCRRequest(copyRecord, options), nil)
	if !errors.Is(err, errStudyOCRPhaseComplete) {
		return nil, err
	}
	return pages, nil
}

func (h *Handler) saveStudyOCRCheckpoint(
	ctx context.Context,
	topperStore studyTopperStore,
	jobID string,
	copyRecord storage.StudyCopyRecord,
	options studyBatchRunOptions,
	checkpoint analyze.Response,
) error {
	record := studyTopperReviewRecord(checkpoint, studyTopperReviewMeta{
		JobID:      jobID,
		SourcePath: copyRecord.SourcePath,
		ProviderID: options.ProviderID,
		Model:      options.Model,
		Status:     "ocr_ready",
	})
	if err := topperStore.SaveTopperReview(ctx, record); err != nil {
		return err
	}
	return h.syncStudyTopperReviewArtifact(checkpoint)
}

func loadStudyOCRCheckpoint(
	ctx context.Context,
	topperStore studyTopperStore,
	copyRecord storage.StudyCopyRecord,
	forceOCR bool,
) ([]analyze.Page, error) {
	if forceOCR {
		return nil, nil
	}
	record, err := topperStore.GetTopperReview(ctx, copyRecord.ID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	status := strings.ToLower(strings.TrimSpace(record.Status))
	if status != "ocr_ready" && status != "needs_review" {
		return nil, nil
	}
	if record.SourcePath != "" && copyRecord.SourcePath != "" && filepath.Clean(record.SourcePath) != filepath.Clean(copyRecord.SourcePath) {
		return nil, nil
	}
	var review analyze.Response
	if err := json.Unmarshal([]byte(record.ReviewJSON), &review); err != nil {
		return nil, fmt.Errorf("parse OCR checkpoint %s: %w", record.ID, err)
	}
	if len(review.Pages) == 0 {
		return nil, nil
	}
	return review.Pages, nil
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
		progress(core.Units(studyBatchProgressLabel(batch.Stage), batch.Completed+batch.Failed, batch.Total, "copy"))
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
		PDFName:       studyTopperReviewPDFName(review),
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

func studyTopperReviewPDFName(review analyze.Response) string {
	return firstString(copySuggestedPDFName(review.Metadata), review.PDFName)
}

func studyTopperSearchText(review analyze.Response) string {
	var b strings.Builder
	b.WriteString(review.PDFName)
	if review.Metadata != nil {
		b.WriteString("\n")
		b.WriteString(jsonString(review.Metadata))
	}
	for _, page := range review.Pages {
		b.WriteString("\n")
		b.WriteString(page.Text)
	}
	for _, question := range review.Questions {
		b.WriteString("\n")
		b.WriteString(question.Label)
		b.WriteString(" ")
		b.WriteString(question.Title)
		if question.Metadata != nil {
			b.WriteString("\n")
			b.WriteString(jsonString(question.Metadata))
		}
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
	options.Model = strings.TrimSpace(options.Model)
	options.OCRModel = strings.TrimSpace(options.OCRModel)
	options.QuestionModel = strings.TrimSpace(options.QuestionModel)
	options.BoundaryModel = strings.TrimSpace(options.BoundaryModel)
	options.ReportModel = strings.TrimSpace(options.ReportModel)
	if options.Parallelism <= 0 {
		options.Parallelism = defaultStudyBatchParallelism
	}
	if options.Parallelism > maxStudyBatchParallelism {
		options.Parallelism = maxStudyBatchParallelism
	}
	return options
}

func studyBatchWorkerAllocation(parallelism int, copies int) (copyWorkers int, modelWorkers int) {
	if parallelism < 1 {
		parallelism = defaultStudyBatchParallelism
	}
	if copies < 1 {
		return 1, 1
	}
	copyWorkers = minInt(parallelism, copies)
	modelWorkers = parallelism / copyWorkers
	if modelWorkers < 1 {
		modelWorkers = 1
	}
	return copyWorkers, modelWorkers
}

func newStudyAnalysisRequest(
	copyRecord storage.StudyCopyRecord,
	options studyBatchRunOptions,
	ocrPages []analyze.Page,
) analyze.Request {
	workers := options.ModelWorkers
	if workers < 1 {
		workers = 1
	}
	return analyze.Request{
		Model:           options.Model,
		OCRModel:        options.OCRModel,
		QuestionModel:   options.QuestionModel,
		BoundaryModel:   options.BoundaryModel,
		ReportModel:     options.ReportModel,
		Path:            copyRecord.SourcePath,
		DPI:             defaultStudyBatchDPI,
		RenderWorkers:   workers,
		Workers:         workers,
		OCRBatchSize:    1,
		OCRInputMode:    analyze.OCRInputModeImages,
		QuestionSplit:   true,
		QuestionWorkers: workers,
		ReviewID:        copyRecord.ID,
		ForceOCR:        options.ForceOCR && len(ocrPages) == 0,
		OCRPages:        ocrPages,
	}
}

func newStudyOCRRequest(copyRecord storage.StudyCopyRecord, options studyBatchRunOptions) analyze.Request {
	req := newStudyAnalysisRequest(copyRecord, options, nil)
	req.Model = options.OCRModel
	req.QuestionModel = ""
	req.BoundaryModel = ""
	req.ReportModel = ""
	req.ForceOCR = options.ForceOCR
	return req
}

func (h *Handler) resolveStudyBatchRunOptions(
	ctx context.Context,
	options studyBatchRunOptions,
) (studyBatchRunOptions, provider.Provider, error) {
	options = normalizedStudyBatchRunOptions(options)
	p, ok := h.runtime.ProviderFor(options.ProviderID)
	if !ok {
		return options, nil, core.ErrProviderNotFound
	}
	if options.Model != "" {
		options = fillStudyBatchModels(options, options.Model)
		return validateStudyBatchModelsLoaded(ctx, options, p)
	}
	if model := firstString(
		options.QuestionModel,
		options.BoundaryModel,
		options.ReportModel,
		options.OCRModel,
	); model != "" {
		options = fillStudyBatchModels(options, model)
		return validateStudyBatchModelsLoaded(ctx, options, p)
	}
	models, err := listLoadedStudyModels(ctx, p)
	if err != nil {
		if options.ProviderID == defaultStudyBatchProviderID {
			return options, nil, fmt.Errorf("LM Studio is unavailable: start its local server, load a vision-capable model, and retry: %w", err)
		}
		return options, nil, fmt.Errorf("list models from provider %q: %w", options.ProviderID, err)
	}
	for _, model := range models {
		if modelID := strings.TrimSpace(model.ID); modelID != "" {
			options = fillStudyBatchModels(options, modelID)
			return validateStudyBatchModelsLoaded(ctx, options, p)
		}
	}
	if options.ProviderID == defaultStudyBatchProviderID {
		return options, nil, errors.New("LM Studio returned no loaded models; load a vision-capable model in LM Studio and retry")
	}
	return options, nil, fmt.Errorf("provider %q returned no models; load or select a model and retry", options.ProviderID)
}

func listLoadedStudyModels(ctx context.Context, p provider.Provider) ([]provider.Model, error) {
	if lister, ok := p.(provider.LoadedModelLister); ok {
		return lister.ListLoadedModels(ctx)
	}
	return p.ListModels(ctx)
}

func validateStudyBatchModelsLoaded(
	ctx context.Context,
	options studyBatchRunOptions,
	p provider.Provider,
) (studyBatchRunOptions, provider.Provider, error) {
	lister, ok := p.(provider.LoadedModelLister)
	if !ok {
		return options, p, nil
	}
	models, err := lister.ListLoadedModels(ctx)
	if err != nil {
		return options, p, fmt.Errorf("check loaded models for provider %q: %w", options.ProviderID, err)
	}
	loaded := make(map[string]bool, len(models))
	for _, model := range models {
		if modelID := strings.ToLower(strings.TrimSpace(model.ID)); modelID != "" {
			loaded[modelID] = true
		}
	}
	required := dedupeStrings([]string{
		options.QuestionModel,
		options.BoundaryModel,
		options.ReportModel,
	})
	missing := make([]string, 0, len(required))
	for _, model := range required {
		if !loaded[strings.ToLower(model)] {
			missing = append(missing, model)
		}
	}
	if len(missing) == 0 {
		return options, p, nil
	}
	if options.ProviderID == defaultStudyBatchProviderID {
		return options, p, fmt.Errorf(
			"LM Studio model(s) are not loaded or unavailable: %s; load them in LM Studio and retry",
			strings.Join(missing, ", "),
		)
	}
	return options, p, fmt.Errorf(
		"provider %q model(s) are unavailable: %s",
		options.ProviderID,
		strings.Join(missing, ", "),
	)
}

func fillStudyBatchModels(options studyBatchRunOptions, fallback string) studyBatchRunOptions {
	options.OCRModel = firstString(options.OCRModel, fallback)
	options.QuestionModel = firstString(options.QuestionModel, fallback)
	options.BoundaryModel = firstString(options.BoundaryModel, options.QuestionModel, fallback)
	options.ReportModel = firstString(options.ReportModel, options.QuestionModel, fallback)
	options.Model = firstString(options.Model, options.QuestionModel, options.ReportModel, options.OCRModel, fallback)
	return options
}

func sameStudyModel(a string, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func studyBatchProgressLabel(stage string) string {
	if stage == "metadata" {
		return "generating metadata"
	}
	return "analyzing topper PDFs"
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
