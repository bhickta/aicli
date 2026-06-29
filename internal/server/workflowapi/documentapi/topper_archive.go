package documentapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/analyze"
	"github.com/bhickta/aicli/internal/workflow/ocr"
)

type topperReviewStore interface {
	SaveTopperReview(ctx context.Context, record storage.TopperReviewRecord) error
	GetTopperReview(ctx context.Context, id string) (storage.TopperReviewRecord, error)
	ListTopperReviews(ctx context.Context, opts storage.TopperReviewListOptions) ([]storage.TopperReviewRecord, error)
}

type topperReviewDeleter interface {
	DeleteTopperReview(ctx context.Context, id string) error
}

type topperReviewMeta struct {
	JobID      string
	SourcePath string
	ProviderID string
	Model      string
	Status     string
}

func (h *Handler) topperStore(w http.ResponseWriter) (topperReviewStore, bool) {
	store, ok := h.runtime.Store().(topperReviewStore)
	if !ok {
		core.WriteError(w, http.StatusNotImplemented, fmt.Errorf("topper review archive is not supported by this store"))
		return nil, false
	}
	return store, true
}

func (h *Handler) listTopperReviews(w http.ResponseWriter, r *http.Request) {
	store, ok := h.topperStore(w)
	if !ok {
		return
	}
	if err := h.backfillTopperReviews(r.Context(), store); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	records, err := store.ListTopperReviews(r.Context(), storage.TopperReviewListOptions{
		Query:  r.URL.Query().Get("query"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"reviews": records})
}

func (h *Handler) getTopperReview(w http.ResponseWriter, r *http.Request) {
	record, ok := h.readTopperReviewRecord(w, r)
	if !ok {
		return
	}
	review, err := decodeTopperReview(record)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"record": record, "review": review})
}

func (h *Handler) updateTopperReview(w http.ResponseWriter, r *http.Request) {
	store, ok := h.topperStore(w)
	if !ok {
		return
	}
	record, ok := h.readTopperReviewRecord(w, r)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[analyze.Response](w, r)
	if !ok {
		return
	}
	if req.ReviewID == "" {
		req.ReviewID = record.ID
	}
	if req.PDFName == "" {
		req.PDFName = record.PDFName
	}
	if err := h.saveTopperReviewRecord(r.Context(), store, req, topperReviewMeta{
		JobID:      record.JobID,
		SourcePath: record.SourcePath,
		ProviderID: record.ProviderID,
		Model:      record.Model,
		Status:     "edited",
	}, record.CreatedAt); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"review": req})
}

func (h *Handler) deleteTopperReview(w http.ResponseWriter, r *http.Request) {
	store, ok := h.topperStore(w)
	if !ok {
		return
	}
	deleter, ok := store.(topperReviewDeleter)
	if !ok {
		core.WriteError(w, http.StatusNotImplemented, fmt.Errorf("topper review deletion is not supported by this store"))
		return
	}
	record, ok := h.readTopperReviewRecord(w, r)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		DeletePDF bool `json:"delete_pdf"`
	}](w, r)
	if !ok {
		return
	}
	result, err := h.deleteTopperReviewFiles(record, req.DeletePDF)
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	if err := deleter.DeleteTopperReview(r.Context(), record.ID); err != nil && !errors.Is(err, storage.ErrNotFound) {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if jobDeleter, ok := h.runtime.Store().(storage.JobDeleter); ok && record.JobID != "" {
		if err := jobDeleter.DeleteJob(r.Context(), record.JobID); err != nil && !errors.Is(err, storage.ErrNotFound) {
			core.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		result.DeletedJob = true
	}
	core.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) rerunTopperReview(w http.ResponseWriter, r *http.Request) {
	record, ok := h.readTopperReviewRecord(w, r)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		ProviderID         string `json:"provider_id"`
		OCRProviderID      string `json:"ocr_provider_id"`
		QuestionProviderID string `json:"question_provider_id"`
		ReportProviderID   string `json:"report_provider_id"`
		analyze.ReprocessRequest
	}](w, r)
	if !ok {
		return
	}
	if (req.Action == "ocr" || req.Action == "all") && !recordHasPageImages(record) {
		core.WriteError(w, http.StatusBadRequest, fmt.Errorf("saved page images are missing; run a fresh Topper copy analysis or PDF OCR before OCR rerun"))
		return
	}
	ocrProviderID := firstProviderID(req.OCRProviderID, req.ProviderID, record.ProviderID)
	questionProviderID := firstProviderID(req.QuestionProviderID, req.ProviderID, record.ProviderID, ocrProviderID)
	reportProviderID := firstProviderID(req.ReportProviderID, req.ProviderID, record.ProviderID, ocrProviderID)
	model := firstModel(req.Model, record.Model)
	ocrModel := firstModel(req.OCRModel, model)
	questionModel := firstModel(req.QuestionModel, model)
	reportModel := firstModel(req.ReportModel, model)
	ocrProvider, found := h.runtime.ProviderOrError(w, ocrProviderID)
	if !found {
		return
	}
	questionProvider, found := h.runtime.ProviderOrError(w, questionProviderID)
	if !found {
		return
	}
	reportProvider, found := h.runtime.ProviderOrError(w, reportProviderID)
	if !found {
		return
	}
	job := core.NewJob("topper-review-rerun", record.ID)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		if req.UnloadModels {
			defer h.unloadProviderModels(context.Background(), []providerModelUse{
				{provider: ocrProvider, model: ocrModel},
				{provider: questionProvider, model: questionModel},
				{provider: reportProvider, model: reportModel},
			})
		}
		store, ok := h.runtime.Store().(topperReviewStore)
		if !ok {
			return nil, fmt.Errorf("topper review archive is not supported by this store")
		}
		latest, err := store.GetTopperReview(ctx, record.ID)
		if err != nil {
			return nil, err
		}
		review, err := decodeTopperReview(latest)
		if err != nil {
			return nil, err
		}
		req.Model = model
		req.OCRModel = ocrModel
		req.QuestionModel = questionModel
		req.ReportModel = reportModel
		result, err := analyze.New(
			h.runtime.Settings().Tools,
			tool.ExecRunner{},
			ocrProvider,
			analyze.WithQuestionProvider(questionProvider),
			analyze.WithReportProvider(reportProvider),
			analyze.WithLogger(h.runtime.Logger()),
		).ReprocessReview(ctx, review, req.ReprocessRequest, func(stage string, completed int, total int, label string) {
			progress(core.Units(stage, completed, total, label))
		})
		if err != nil {
			return nil, err
		}
		if err := h.saveTopperReviewRecord(ctx, store, result, topperReviewMeta{
			JobID:      job.ID,
			SourcePath: latest.SourcePath,
			ProviderID: ocrProviderID,
			Model:      ocrModel,
			Status:     "ready",
		}, latest.CreatedAt); err != nil {
			return nil, err
		}
		return result, nil
	})
}

type deleteTopperReviewResult struct {
	DeletedReviewAssets []string `json:"deleted_review_assets"`
	DeletedOCRAssets    []string `json:"deleted_ocr_assets"`
	DeletedPDF          string   `json:"deleted_pdf"`
	SkippedPDF          string   `json:"skipped_pdf"`
	DeletedJob          bool     `json:"deleted_job"`
}

func (h *Handler) deleteTopperReviewFiles(record storage.TopperReviewRecord, deletePDF bool) (deleteTopperReviewResult, error) {
	result := deleteTopperReviewResult{}
	if h.runtime.DataDir() != "" {
		reviewDir := filepath.Join(h.runtime.DataDir(), "artifacts", "topper-copy", record.ID)
		if removeDirIfExists(reviewDir) {
			result.DeletedReviewAssets = append(result.DeletedReviewAssets, reviewDir)
		}
		if record.JobID != "" {
			ocrDir := filepath.Join(h.runtime.DataDir(), "artifacts", "pdf-ocr", record.JobID)
			if removeDirIfExists(ocrDir) {
				result.DeletedOCRAssets = append(result.DeletedOCRAssets, ocrDir)
			}
		}
	}
	if deletePDF {
		deleted, skipped, err := h.deleteUploadedPDF(record.SourcePath)
		if err != nil {
			return result, err
		}
		result.DeletedPDF = deleted
		result.SkippedPDF = skipped
	}
	return result, nil
}

func removeDirIfExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return os.RemoveAll(path) == nil
}

func (h *Handler) deleteUploadedPDF(path string) (string, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", "No source PDF path saved for this review.", nil
	}
	if h.runtime.DataDir() == "" {
		return "", path, fmt.Errorf("data directory is not configured")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", path, err
	}
	uploadRoot, err := filepath.Abs(filepath.Join(h.runtime.DataDir(), "uploads"))
	if err != nil {
		return "", path, err
	}
	rel, err := filepath.Rel(uploadRoot, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", "Skipped PDF outside AICLI uploads: " + path, nil
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return "", path, err
	}
	return abs, "", nil
}

func recordHasPageImages(record storage.TopperReviewRecord) bool {
	review, err := decodeTopperReview(record)
	if err != nil {
		return false
	}
	for _, page := range review.Pages {
		if strings.TrimSpace(page.Path) == "" {
			return false
		}
	}
	return len(review.Pages) > 0
}

func (h *Handler) readTopperReviewRecord(w http.ResponseWriter, r *http.Request) (storage.TopperReviewRecord, bool) {
	store, ok := h.topperStore(w)
	if !ok {
		return storage.TopperReviewRecord{}, false
	}
	record, err := store.GetTopperReview(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		core.WriteError(w, http.StatusNotFound, err)
		return storage.TopperReviewRecord{}, false
	}
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return storage.TopperReviewRecord{}, false
	}
	return record, true
}

func (h *Handler) saveTopperReview(ctx context.Context, review analyze.Response, meta topperReviewMeta) error {
	store, ok := h.runtime.Store().(topperReviewStore)
	if !ok {
		return nil
	}
	return h.saveTopperReviewRecord(ctx, store, review, meta, time.Time{})
}

func (h *Handler) rejectAlreadyProcessedPDF(w http.ResponseWriter, r *http.Request, sourcePath string) bool {
	record, found, err := h.findProcessedPDFByName(r.Context(), sourcePath)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return true
	}
	if !found {
		return false
	}
	core.WriteError(w, http.StatusConflict, fmt.Errorf("PDF filename already processed as %s (%s)", record.ID, record.PDFName))
	return true
}

func (h *Handler) findProcessedPDFByName(ctx context.Context, sourcePath string) (storage.TopperReviewRecord, bool, error) {
	store, ok := h.runtime.Store().(topperReviewStore)
	if !ok {
		return storage.TopperReviewRecord{}, false, nil
	}
	if err := h.backfillTopperReviews(ctx, store); err != nil {
		return storage.TopperReviewRecord{}, false, err
	}
	filename := strings.ToLower(filepath.Base(sourcePath))
	if filename == "." || filename == "" {
		return storage.TopperReviewRecord{}, false, nil
	}
	records, err := store.ListTopperReviews(ctx, storage.TopperReviewListOptions{Query: filename, Limit: 200})
	if err != nil {
		return storage.TopperReviewRecord{}, false, err
	}
	for _, record := range records {
		if strings.ToLower(record.PDFName) == filename || strings.ToLower(filepath.Base(record.SourcePath)) == filename {
			return record, true, nil
		}
	}
	return storage.TopperReviewRecord{}, false, nil
}

func (h *Handler) saveTopperReviewRecord(ctx context.Context, store topperReviewStore, review analyze.Response, meta topperReviewMeta, createdAt time.Time) error {
	if err := store.SaveTopperReview(ctx, buildTopperReviewRecord(review, meta, createdAt)); err != nil {
		return err
	}
	return h.syncTopperReviewArtifact(review)
}

func (h *Handler) syncTopperReviewArtifact(review analyze.Response) error {
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

func decodeTopperReview(record storage.TopperReviewRecord) (analyze.Response, error) {
	var review analyze.Response
	if err := json.Unmarshal([]byte(record.ReviewJSON), &review); err != nil {
		return analyze.Response{}, err
	}
	return review, nil
}

func buildTopperReviewRecord(review analyze.Response, meta topperReviewMeta, createdAt time.Time) storage.TopperReviewRecord {
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
		UnclearCount:  topperUnclearCount(review),
		Status:        meta.Status,
		ReviewJSON:    string(data),
		SearchText:    topperSearchText(review),
		CreatedAt:     createdAt,
	}
}

func topperUnclearCount(review analyze.Response) int {
	total := 0
	for _, page := range review.Pages {
		total += page.UnclearCount
	}
	return total
}

func topperSearchText(review analyze.Response) string {
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

func (h *Handler) backfillTopperReviews(ctx context.Context, store topperReviewStore) error {
	if err := h.backfillTopperReviewJobs(ctx, store); err != nil {
		return err
	}
	return h.backfillTopperReviewArtifacts(ctx, store)
}

func (h *Handler) backfillTopperReviewJobs(ctx context.Context, store topperReviewStore) error {
	jobs, err := h.runtime.Store().ListJobs(ctx)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job.Output == "" {
			continue
		}
		review, ok := decodeTopperReviewOutput(job.Output)
		if !ok {
			review, ok = decodePDFOCRReviewOutput(job)
			if !ok {
				continue
			}
		}
		status := "ready"
		if strings.HasPrefix(review.ReviewID, "ocr-") {
			status = "ocr-only"
		}
		if err := store.SaveTopperReview(ctx, buildTopperReviewRecord(review, topperReviewMeta{
			JobID:      job.ID,
			SourcePath: job.Input,
			Status:     status,
		}, job.CreatedAt)); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) backfillTopperReviewArtifacts(ctx context.Context, store topperReviewStore) error {
	if h.runtime.DataDir() == "" {
		return nil
	}
	root := filepath.Join(h.runtime.DataDir(), "artifacts", "topper-copy")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "review.json")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		review, ok := decodeTopperReviewOutput(string(data))
		if !ok {
			continue
		}
		info, _ := os.Stat(path)
		createdAt := time.Time{}
		if info != nil {
			createdAt = info.ModTime().UTC()
		}
		if err := store.SaveTopperReview(ctx, buildTopperReviewRecord(review, topperReviewMeta{
			Status: "ready",
		}, createdAt)); err != nil {
			return err
		}
	}
	return nil
}

func decodeTopperReviewOutput(output string) (analyze.Response, bool) {
	var review analyze.Response
	if err := json.Unmarshal([]byte(output), &review); err != nil {
		return analyze.Response{}, false
	}
	if review.Kind != "topper_copy_review" || review.ReviewID == "" {
		return analyze.Response{}, false
	}
	return review, true
}

func decodePDFOCRReviewOutput(job storage.Job) (analyze.Response, bool) {
	if job.Type != "pdf-ocr" || job.Status != storage.JobStatusCompleted {
		return analyze.Response{}, false
	}
	var output ocr.Response
	if err := json.Unmarshal([]byte(job.Output), &output); err != nil {
		return analyze.Response{}, false
	}
	review := topperReviewFromOCR(job.ID, job.Input, output)
	if len(review.Pages) == 0 {
		return analyze.Response{}, false
	}
	return review, true
}

func topperReviewFromOCR(jobID string, sourcePath string, output ocr.Response) analyze.Response {
	pages := ocrResponsePages(output)
	if len(pages) == 0 {
		pages = ocrMarkdownPages(output.Markdown)
	}
	return analyze.Response{
		Kind:       "topper_copy_review",
		ReviewID:   "ocr-" + jobID,
		PDFName:    filepath.Base(sourcePath),
		SourceMode: analyze.OCRInputModeImages,
		Pages:      pages,
		Questions:  pageReviewQuestions(pages),
		Report:     "Imported from PDF OCR output. Run page or full review from Study Archive for AI analysis.",
	}
}

func ocrResponsePages(output ocr.Response) []analyze.Page {
	pages := make([]analyze.Page, 0, len(output.Pages))
	for index, page := range output.Pages {
		body := strings.TrimSpace(page.Markdown)
		if body == "" {
			continue
		}
		number := index + 1
		if parsed, _, ok := parseOCRPageMarker(fmt.Sprintf("<!-- Page %d %s -->", number, page.Name)); ok {
			number = parsed
		}
		name := strings.TrimSpace(page.Name)
		if name == "" {
			name = fmt.Sprintf("page-%d", number)
		}
		pages = append(pages, analyze.Page{
			Number:       number,
			Name:         name,
			Path:         page.Path,
			ImageURL:     page.ImageURL,
			Text:         body,
			UnclearCount: strings.Count(strings.ToLower(body), "[unclear]"),
			Verified:     false,
		})
	}
	return pages
}

func ocrMarkdownPages(markdown string) []analyze.Page {
	var pages []analyze.Page
	currentNumber := 0
	currentName := ""
	var text strings.Builder

	flush := func() {
		body := strings.TrimSpace(text.String())
		if currentNumber == 0 || body == "" {
			text.Reset()
			return
		}
		lowerBody := strings.ToLower(body)
		pages = append(pages, analyze.Page{
			Number:       currentNumber,
			Name:         currentName,
			Text:         body,
			UnclearCount: strings.Count(lowerBody, "[unclear]"),
			Verified:     false,
		})
		text.Reset()
	}

	for _, line := range strings.Split(markdown, "\n") {
		number, name, ok := parseOCRPageMarker(strings.TrimSpace(line))
		if ok {
			flush()
			currentNumber = number
			currentName = name
			continue
		}
		if currentNumber == 0 {
			continue
		}
		text.WriteString(line)
		text.WriteString("\n")
	}
	flush()

	if len(pages) > 0 {
		return pages
	}
	body := strings.TrimSpace(markdown)
	if body == "" {
		return nil
	}
	return []analyze.Page{{
		Number:       1,
		Name:         "page-1",
		Text:         body,
		UnclearCount: strings.Count(strings.ToLower(body), "[unclear]"),
		Verified:     false,
	}}
}

func parseOCRPageMarker(line string) (int, string, bool) {
	if !strings.HasPrefix(line, "<!-- Page ") || !strings.HasSuffix(line, " -->") {
		return 0, "", false
	}
	content := strings.TrimSuffix(strings.TrimPrefix(line, "<!-- Page "), " -->")
	parts := strings.Fields(content)
	if len(parts) == 0 {
		return 0, "", false
	}
	number, err := strconv.Atoi(parts[0])
	if err != nil || number <= 0 {
		return 0, "", false
	}
	name := fmt.Sprintf("page-%d", number)
	if len(parts) > 1 {
		name = parts[1]
	}
	return number, name, true
}

func pageReviewQuestions(pages []analyze.Page) []analyze.Question {
	questions := make([]analyze.Question, 0, len(pages))
	for _, page := range pages {
		questions = append(questions, analyze.Question{
			ID:             fmt.Sprintf("page-%d", page.Number),
			Label:          fmt.Sprintf("Page %d OCR", page.Number),
			Title:          "OCR text",
			AnswerMarkdown: page.Text,
			SourcePages:    []int{page.Number},
			Status:         "needs review",
		})
	}
	return questions
}
