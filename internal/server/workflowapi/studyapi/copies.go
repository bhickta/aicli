package studyapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

type studyStore interface {
	SaveStudyCopy(context.Context, storage.StudyCopyRecord) error
	GetStudyCopy(context.Context, string) (storage.StudyCopyRecord, error)
	ListStudyCopies(context.Context, storage.StudyCopyListOptions) ([]storage.StudyCopyRecord, error)
	SaveStudyPage(context.Context, storage.StudyPageRecord) error
	ListStudyPages(context.Context, string) ([]storage.StudyPageRecord, error)
	SaveStudyQuestion(context.Context, storage.StudyQuestionRecord) error
	ListStudyQuestions(context.Context, string) ([]storage.StudyQuestionRecord, error)
	SaveStudyAnalysis(context.Context, storage.StudyAnalysisRecord) error
	ListStudyAnalyses(context.Context, string) ([]storage.StudyAnalysisRecord, error)
	SaveStudyBatch(context.Context, storage.StudyBatchRecord) error
	SaveStudyBatchItem(context.Context, storage.StudyBatchItemRecord) error
	ListStudyBatchItems(context.Context, string) ([]storage.StudyBatchItemRecord, error)
}

type studyTopperStore interface {
	ListTopperReviews(context.Context, storage.TopperReviewListOptions) ([]storage.TopperReviewRecord, error)
	GetTopperReview(context.Context, string) (storage.TopperReviewRecord, error)
}

func (h *Handler) listStudyCopies(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	_ = h.backfillStudyCopies(r.Context(), store)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	copies, err := store.ListStudyCopies(r.Context(), storage.StudyCopyListOptions{
		Query:  r.URL.Query().Get("query"),
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"copies": copies})
}

func (h *Handler) getStudyCopy(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if err := h.backfillStudyCopyID(r.Context(), store, id); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	copyRecord, err := store.GetStudyCopy(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	pages, err := store.ListStudyPages(r.Context(), id)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	questions, err := store.ListStudyQuestions(r.Context(), id)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	analyses, err := store.ListStudyAnalyses(r.Context(), id)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{
		"copy": copyRecord, "pages": pages, "questions": questions, "analyses": analyses,
	})
}

func (h *Handler) updateStudyCopy(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	current, err := store.GetStudyCopy(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	req, ok := core.DecodeJSON[storage.StudyCopyRecord](w, r)
	if !ok {
		return
	}
	req.ID = current.ID
	req.SourcePath = firstString(req.SourcePath, current.SourcePath)
	req.SourceHash = firstString(req.SourceHash, current.SourceHash)
	req.PDFName = firstString(req.PDFName, current.PDFName)
	req.PageCount = firstInt(req.PageCount, current.PageCount)
	req.QuestionCount = firstInt(req.QuestionCount, current.QuestionCount)
	req.UnclearCount = firstInt(req.UnclearCount, current.UnclearCount)
	req.CreatedAt = current.CreatedAt
	if err := store.SaveStudyCopy(r.Context(), req); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	updated, _ := store.GetStudyCopy(r.Context(), current.ID)
	core.WriteJSON(w, http.StatusOK, map[string]any{"copy": updated})
}

func (h *Handler) importStudyCopies(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		Paths      []string `json:"paths"`
		FolderPath string   `json:"folder_path"`
		Recursive  bool     `json:"recursive"`
	}](w, r)
	if !ok {
		return
	}
	paths, err := collectStudyPDFs(req.Paths, req.FolderPath, req.Recursive)
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	var copies []storage.StudyCopyRecord
	for _, path := range paths {
		copyRecord := importedStudyCopy(path)
		if err := store.SaveStudyCopy(r.Context(), copyRecord); err != nil {
			core.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		copies = append(copies, copyRecord)
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"copies": copies})
}

func (h *Handler) startStudyStage(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		Stage string `json:"stage"`
	}](w, r)
	if !ok {
		return
	}
	copyID := r.PathValue("id")
	copyRecord, err := store.GetStudyCopy(r.Context(), copyID)
	if err != nil {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	stage := normalizedStudyStage(req.Stage)
	batch := storage.StudyBatchRecord{
		ID:        "study-batch-" + time.Now().UTC().Format("20060102150405.000000000"),
		Status:    "queued",
		Stage:     stage,
		Total:     1,
		Completed: 0,
	}
	if err := store.SaveStudyBatch(r.Context(), batch); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	_ = store.SaveStudyBatchItem(r.Context(), storage.StudyBatchItemRecord{
		BatchID: batch.ID, CopyID: copyRecord.ID, Stage: stage, Status: "queued",
	})
	core.WriteJSON(w, http.StatusAccepted, map[string]any{"batch": batch})
}

func (h *Handler) startStudyBatch(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		CopyIDs []string `json:"copy_ids"`
		Stage   string   `json:"stage"`
	}](w, r)
	if !ok {
		return
	}
	stage := normalizedStudyStage(req.Stage)
	batch := storage.StudyBatchRecord{
		ID:     "study-batch-" + time.Now().UTC().Format("20060102150405.000000000"),
		Status: "queued",
		Stage:  stage,
		Total:  len(req.CopyIDs),
	}
	if err := store.SaveStudyBatch(r.Context(), batch); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	for _, copyID := range req.CopyIDs {
		_ = store.SaveStudyBatchItem(r.Context(), storage.StudyBatchItemRecord{
			BatchID: batch.ID, CopyID: copyID, Stage: stage, Status: "queued",
		})
	}
	items, _ := store.ListStudyBatchItems(r.Context(), batch.ID)
	core.WriteJSON(w, http.StatusAccepted, map[string]any{"batch": batch, "items": items})
}

func (h *Handler) studyStore(w http.ResponseWriter) (studyStore, bool) {
	store, ok := h.runtime.Store().(studyStore)
	if !ok {
		core.WriteError(w, http.StatusNotImplemented, fmt.Errorf("study archive is not supported by this store"))
		return nil, false
	}
	return store, true
}

func (h *Handler) backfillStudyCopies(ctx context.Context, store studyStore) error {
	topperStore, ok := h.runtime.Store().(studyTopperStore)
	if !ok {
		return nil
	}
	records, err := topperStore.ListTopperReviews(ctx, storage.TopperReviewListOptions{Limit: 500})
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := saveStudyFromTopperRecord(ctx, store, record); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) backfillStudyCopyID(ctx context.Context, store studyStore, id string) error {
	if _, err := store.GetStudyCopy(ctx, id); err == nil {
		return nil
	}
	topperStore, ok := h.runtime.Store().(studyTopperStore)
	if !ok {
		return nil
	}
	record, err := topperStore.GetTopperReview(ctx, id)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return saveStudyFromTopperRecord(ctx, store, record)
}

func saveStudyFromTopperRecord(ctx context.Context, store studyStore, record storage.TopperReviewRecord) error {
	var review analyze.Response
	if err := json.Unmarshal([]byte(record.ReviewJSON), &review); err != nil {
		return nil
	}
	copyRecord := storage.StudyCopyRecord{
		ID:             record.ID,
		SourcePath:     record.SourcePath,
		PDFName:        firstString(record.PDFName, review.PDFName),
		PageCount:      len(review.Pages),
		QuestionCount:  len(review.Questions),
		UnclearCount:   record.UnclearCount,
		Status:         record.Status,
		RenderStatus:   "ready",
		OCRStatus:      statusFromCount(len(review.Pages)),
		QuestionStatus: statusFromCount(len(review.Questions)),
		AnalysisStatus: "pending",
		ReportStatus:   statusFromText(review.Report),
		CreatedAt:      record.CreatedAt,
	}
	if err := store.SaveStudyCopy(ctx, copyRecord); err != nil {
		return err
	}
	for _, page := range review.Pages {
		if err := store.SaveStudyPage(ctx, storage.StudyPageRecord{
			CopyID:       record.ID,
			PageNumber:   page.Number,
			Name:         page.Name,
			ImagePath:    page.Path,
			ImageURL:     page.ImageURL,
			OCRText:      page.Text,
			RawOCR:       page.Text,
			Status:       "ready",
			UnclearCount: page.UnclearCount,
			Verified:     page.Verified,
			CreatedAt:    record.CreatedAt,
		}); err != nil {
			return err
		}
	}
	for index, question := range review.Questions {
		if err := store.SaveStudyQuestion(ctx, storage.StudyQuestionRecord{
			ID:          firstString(question.ID, fmt.Sprintf("%s-q%d", record.ID, index+1)),
			CopyID:      record.ID,
			QuestionNo:  inferQuestionNo(question.Label, index+1),
			Label:       question.Label,
			PromptText:  question.Title,
			AnswerText:  question.AnswerMarkdown,
			SourcePages: question.SourcePages,
			Status:      firstString(question.Status, "ready"),
			CreatedAt:   record.CreatedAt,
		}); err != nil {
			return err
		}
	}
	if strings.TrimSpace(review.Report) != "" {
		return store.SaveStudyAnalysis(ctx, storage.StudyAnalysisRecord{
			ID:           record.ID + "-report",
			CopyID:       record.ID,
			ScopeType:    "copy",
			ScopeID:      record.ID,
			DimensionKey: "report",
			ProviderID:   record.ProviderID,
			Model:        record.Model,
			ResultJSON:   jsonString(map[string]string{"report": review.Report}),
			CreatedAt:    record.CreatedAt,
		})
	}
	return nil
}

func collectStudyPDFs(paths []string, folderPath string, recursive bool) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || strings.ToLower(filepath.Ext(path)) != ".pdf" || seen[path] {
			return
		}
		seen[path] = true
		out = append(out, path)
	}
	for _, path := range paths {
		add(path)
	}
	if folderPath != "" {
		walkFn := func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil {
				return err
			}
			if info.IsDir() && path != folderPath && !recursive {
				return filepath.SkipDir
			}
			if !info.IsDir() {
				add(path)
			}
			return nil
		}
		if err := filepath.Walk(folderPath, walkFn); err != nil {
			return nil, err
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no PDF files found")
	}
	return out, nil
}

func importedStudyCopy(path string) storage.StudyCopyRecord {
	sum := sha256.Sum256([]byte(path))
	id := "study-" + hex.EncodeToString(sum[:])[:16]
	status := "imported"
	if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%d", path, stat.Size(), stat.ModTime().UnixNano())))
		id = "study-" + hex.EncodeToString(hash[:])[:16]
	}
	return storage.StudyCopyRecord{
		ID:             id,
		SourcePath:     path,
		SourceHash:     id,
		PDFName:        filepath.Base(path),
		Status:         status,
		RenderStatus:   "pending",
		OCRStatus:      "pending",
		QuestionStatus: "pending",
		AnalysisStatus: "pending",
		ReportStatus:   "pending",
	}
}

var questionNoPattern = regexp.MustCompile(`(?i)q\.?\s*([0-9]+)`)

func inferQuestionNo(label string, fallback int) int {
	match := questionNoPattern.FindStringSubmatch(label)
	if len(match) < 2 {
		return fallback
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return fallback
	}
	return value
}

func normalizedStudyStage(stage string) string {
	stage = strings.ToLower(strings.TrimSpace(stage))
	switch stage {
	case "render", "ocr", "questions", "analysis", "report", "all":
		return stage
	default:
		return "all"
	}
}

func statusFromCount(count int) string {
	if count > 0 {
		return "ready"
	}
	return "pending"
}

func statusFromText(text string) string {
	if strings.TrimSpace(text) != "" {
		return "ready"
	}
	return "pending"
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func jsonString(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
