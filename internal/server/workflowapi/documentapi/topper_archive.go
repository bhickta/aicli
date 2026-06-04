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
)

type topperReviewStore interface {
	SaveTopperReview(ctx context.Context, record storage.TopperReviewRecord) error
	GetTopperReview(ctx context.Context, id string) (storage.TopperReviewRecord, error)
	ListTopperReviews(ctx context.Context, opts storage.TopperReviewListOptions) ([]storage.TopperReviewRecord, error)
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

func (h *Handler) rerunTopperReview(w http.ResponseWriter, r *http.Request) {
	record, ok := h.readTopperReviewRecord(w, r)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		analyze.ReprocessRequest
	}](w, r)
	if !ok {
		return
	}
	providerID := strings.TrimSpace(req.ProviderID)
	if providerID == "" {
		providerID = record.ProviderID
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = record.Model
	}
	p, found := h.runtime.ProviderOrError(w, providerID)
	if !found {
		return
	}
	job := core.NewJob("topper-review-rerun", record.ID)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
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
		result, err := analyze.New(h.runtime.Settings().Tools, tool.ExecRunner{}, p).ReprocessReview(ctx, review, req.ReprocessRequest, func(stage string, completed int, total int, label string) {
			progress(core.Units(stage, completed, total, label))
		})
		if err != nil {
			return nil, err
		}
		if err := h.saveTopperReviewRecord(ctx, store, result, topperReviewMeta{
			JobID:      job.ID,
			SourcePath: latest.SourcePath,
			ProviderID: providerID,
			Model:      model,
			Status:     "ready",
		}, latest.CreatedAt); err != nil {
			return nil, err
		}
		return result, nil
	})
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
	var output struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(job.Output), &output); err != nil {
		return analyze.Response{}, false
	}
	pages := ocrMarkdownPages(output.Markdown)
	if len(pages) == 0 {
		return analyze.Response{}, false
	}
	return analyze.Response{
		Kind:      "topper_copy_review",
		ReviewID:  "ocr-" + job.ID,
		PDFName:   filepath.Base(job.Input),
		Pages:     pages,
		Questions: pageReviewQuestions(pages),
		Report:    "Imported from PDF OCR output. Run page or full review from Study Archive for AI analysis.",
	}, true
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
