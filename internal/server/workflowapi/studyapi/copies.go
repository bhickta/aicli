package studyapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	ReplaceStudyCopyResult(context.Context, storage.StudyCopyRecord, []storage.StudyPageRecord, []storage.StudyQuestionRecord, []storage.StudyAnalysisRecord) error
	SaveStudyBatch(context.Context, storage.StudyBatchRecord) error
	SaveStudyBatchItem(context.Context, storage.StudyBatchItemRecord) error
	GetStudyBatch(context.Context, string) (storage.StudyBatchRecord, error)
	ListStudyBatches(context.Context, int) ([]storage.StudyBatchRecord, error)
	ListStudyBatchItems(context.Context, string) ([]storage.StudyBatchItemRecord, error)
}

type studyTopperStore interface {
	SaveTopperReview(context.Context, storage.TopperReviewRecord) error
	ListTopperReviews(context.Context, storage.TopperReviewListOptions) ([]storage.TopperReviewRecord, error)
	GetTopperReview(context.Context, string) (storage.TopperReviewRecord, error)
}

func (h *Handler) RegisterRoutes(r *http.ServeMux) {
	r.HandleFunc("GET /api/study/copies", h.listStudyCopies)
	r.HandleFunc("GET /api/study/copies/{id}", h.getStudyCopy)
	r.HandleFunc("POST /api/study/copies/{id}/run", h.runStudyCopy)
	r.HandleFunc("POST /api/study/copies/{id}/sync", h.syncStudyCopy)
	r.HandleFunc("PUT /api/study/copies/{id}", h.updateStudyCopy)
	r.HandleFunc("POST /api/study/imports", h.importStudyCopies)
	r.HandleFunc("POST /api/study/batches", h.startStudyBatch)
	r.HandleFunc("GET /api/study/batches", h.listStudyBatches)
	r.HandleFunc("GET /api/study/batches/{id}", h.getStudyBatch)
	r.HandleFunc("POST /api/study/stages", h.startStudyStage)
}

func (h *Handler) listStudyCopies(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
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
	if synced, err := h.syncStudyCopyFromMatchingTopper(r.Context(), store, copyRecord, false); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	} else if synced {
		copyRecord, err = store.GetStudyCopy(r.Context(), id)
		if err != nil {
			core.WriteError(w, http.StatusInternalServerError, err)
			return
		}
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

func (h *Handler) syncStudyCopy(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	id := r.PathValue("id")
	topperStore, ok := h.runtime.Store().(studyTopperStore)
	if !ok {
		core.WriteError(w, http.StatusInternalServerError, fmt.Errorf("no topper store"))
		return
	}
	existing, existingErr := store.GetStudyCopy(r.Context(), id)
	if existingErr != nil && !errors.Is(existingErr, storage.ErrNotFound) {
		core.WriteError(w, http.StatusInternalServerError, existingErr)
		return
	}
	if payload, ok := decodeStudyCopySyncPayload(w, r); !ok {
		return
	} else if payload.Review != nil {
		if existingErr != nil {
			existing = storage.StudyCopyRecord{}
		}
		if err := h.saveStudyCopySyncPayload(r.Context(), store, topperStore, id, existing, payload); err != nil {
			core.WriteError(w, http.StatusInternalServerError, err)
			return
		}
		h.getStudyCopy(w, r)
		return
	}
	record, err := topperStore.GetTopperReview(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) && existingErr == nil {
		synced, syncErr := h.syncStudyCopyFromMatchingTopper(r.Context(), store, existing, true)
		if syncErr != nil {
			core.WriteError(w, http.StatusInternalServerError, syncErr)
			return
		}
		if !synced {
			core.WriteError(w, http.StatusNotFound, err)
			return
		}
		h.getStudyCopy(w, r)
		return
	}
	if errors.Is(err, storage.ErrNotFound) {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	if existingErr != nil {
		existing = storage.StudyCopyRecord{}
	}
	if err := saveStudyFromTopperRecordAsCopy(r.Context(), store, record, id, existing); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	h.getStudyCopy(w, r)
}

type studyCopySyncPayload struct {
	Review     *analyze.Response `json:"review"`
	ProviderID string            `json:"provider_id"`
	Model      string            `json:"model"`
	SourcePath string            `json:"source_path"`
}

func decodeStudyCopySyncPayload(w http.ResponseWriter, r *http.Request) (studyCopySyncPayload, bool) {
	if r.Body == nil || r.ContentLength == 0 {
		return studyCopySyncPayload{}, true
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return studyCopySyncPayload{}, false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return studyCopySyncPayload{}, true
	}
	var payload studyCopySyncPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return studyCopySyncPayload{}, false
	}
	return payload, true
}

func (h *Handler) saveStudyCopySyncPayload(
	ctx context.Context,
	store studyStore,
	topperStore studyTopperStore,
	copyID string,
	existing storage.StudyCopyRecord,
	payload studyCopySyncPayload,
) error {
	review := *payload.Review
	review.Kind = firstString(review.Kind, "topper_copy_review")
	review.ReviewID = copyID
	review.PDFName = firstString(review.PDFName, existing.PDFName, filepath.Base(firstString(payload.SourcePath, existing.SourcePath)))
	record := studyTopperReviewRecord(review, studyTopperReviewMeta{
		SourcePath: firstString(payload.SourcePath, existing.SourcePath),
		ProviderID: payload.ProviderID,
		Model:      payload.Model,
		Status:     "ready",
	})
	if err := topperStore.SaveTopperReview(ctx, record); err != nil {
		return fmt.Errorf("save topper review %s: %w", record.ID, err)
	}
	return saveStudyFromTopperRecordAsCopy(ctx, store, record, copyID, existing)
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

func (h *Handler) runStudyCopy(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		Model      string `json:"model"`
		ForceOCR   bool   `json:"force_ocr"`
	}](w, r)
	if !ok {
		return
	}
	copyID := strings.TrimSpace(r.PathValue("id"))
	if copyID == "" {
		core.WriteError(w, http.StatusBadRequest, fmt.Errorf("copy id is required"))
		return
	}
	options := normalizedStudyBatchRunOptions(studyBatchRunOptions{
		ProviderID:  req.ProviderID,
		Model:       req.Model,
		Parallelism: 1,
		ForceOCR:    req.ForceOCR,
	})
	batch, copies, ok := h.prepareStudyBatch(w, r, store, []string{copyID}, "all", options)
	if !ok {
		return
	}
	items, _ := store.ListStudyBatchItems(r.Context(), batch.ID)
	h.startStudyBatchJob(w, r, store, batch, items, copies, options)
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

func (h *Handler) getStudyBatch(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	batch, err := store.GetStudyBatch(r.Context(), r.PathValue("id"))
	if errors.Is(err, storage.ErrNotFound) {
		core.WriteError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	items, err := store.ListStudyBatchItems(r.Context(), batch.ID)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"batch": batch, "items": items})
}

func (h *Handler) listStudyBatches(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	batches, err := store.ListStudyBatches(r.Context(), limit)
	if err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, map[string]any{"batches": batches})
}

func (h *Handler) startStudyBatch(w http.ResponseWriter, r *http.Request) {
	store, ok := h.studyStore(w)
	if !ok {
		return
	}
	req, ok := core.DecodeJSON[struct {
		CopyIDs     []string `json:"copy_ids"`
		Stage       string   `json:"stage"`
		ProviderID  string   `json:"provider_id"`
		Model       string   `json:"model"`
		Parallelism int      `json:"parallelism"`
		ForceOCR    bool     `json:"force_ocr"`
	}](w, r)
	if !ok {
		return
	}
	if len(req.CopyIDs) == 0 {
		core.WriteError(w, http.StatusBadRequest, fmt.Errorf("select at least one copy"))
		return
	}
	options := normalizedStudyBatchRunOptions(studyBatchRunOptions{
		ProviderID:  req.ProviderID,
		Model:       req.Model,
		Parallelism: req.Parallelism,
		ForceOCR:    req.ForceOCR,
	})
	batch, copies, ok := h.prepareStudyBatch(w, r, store, req.CopyIDs, req.Stage, options)
	if !ok {
		return
	}
	items, _ := store.ListStudyBatchItems(r.Context(), batch.ID)
	h.startStudyBatchJob(w, r, store, batch, items, copies, options)
}

func (h *Handler) prepareStudyBatch(
	w http.ResponseWriter,
	r *http.Request,
	store studyStore,
	requestedCopyIDs []string,
	requestedStage string,
	options studyBatchRunOptions,
) (storage.StudyBatchRecord, []storage.StudyCopyRecord, bool) {
	stage := normalizedStudyStage(requestedStage)
	if _, ok := h.runtime.ProviderFor(options.ProviderID); !ok {
		core.WriteError(w, http.StatusNotFound, core.ErrProviderNotFound)
		return storage.StudyBatchRecord{}, nil, false
	}
	if stage != "metadata" && !h.studyBatchProviderSupportsDirectPDF(options.ProviderID) {
		core.WriteError(w, http.StatusBadRequest, fmt.Errorf("provider %q does not support direct PDF input", options.ProviderID))
		return storage.StudyBatchRecord{}, nil, false
	}
	copyIDs := dedupeStrings(requestedCopyIDs)
	now := time.Now().UTC()
	batch := storage.StudyBatchRecord{
		ID:          "study-batch-" + now.Format("20060102150405.000000000"),
		Status:      "running",
		Stage:       stage,
		ProviderID:  options.ProviderID,
		Model:       options.Model,
		Parallelism: options.Parallelism,
		ForceRerun:  options.ForceOCR,
		Total:       len(copyIDs),
		StartedAt:   now,
	}
	copies := make([]storage.StudyCopyRecord, 0, len(copyIDs))
	for _, copyID := range copyIDs {
		copyRecord, err := store.GetStudyCopy(r.Context(), copyID)
		if err != nil {
			core.WriteError(w, http.StatusNotFound, fmt.Errorf("study copy %q: %w", copyID, err))
			return storage.StudyBatchRecord{}, nil, false
		}
		copies = append(copies, copyRecord)
	}
	if err := store.SaveStudyBatch(r.Context(), batch); err != nil {
		core.WriteError(w, http.StatusInternalServerError, err)
		return storage.StudyBatchRecord{}, nil, false
	}
	for _, copyID := range copyIDs {
		if err := store.SaveStudyBatchItem(r.Context(), storage.StudyBatchItemRecord{
			BatchID: batch.ID, CopyID: copyID, Stage: stage, Status: "queued",
		}); err != nil {
			core.WriteError(w, http.StatusInternalServerError, err)
			return storage.StudyBatchRecord{}, nil, false
		}
	}
	return batch, copies, true
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
	for _, summary := range records {
		record, err := topperStore.GetTopperReview(ctx, summary.ID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if err := h.backfillStudyFromTopperRecord(ctx, store, record); err != nil {
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
	return saveStudyFromTopperRecordAsCopy(ctx, store, record, record.ID, storage.StudyCopyRecord{})
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
	case "render", "ocr", "questions", "analysis", "report", "metadata", "all":
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
