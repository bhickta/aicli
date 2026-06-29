package studyapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

func (h *Handler) backfillStudyFromTopperRecord(
	ctx context.Context,
	store studyStore,
	record storage.TopperReviewRecord,
) error {
	copyRecord, found, err := findStudyCopyForTopper(ctx, store, record)
	if err != nil {
		return err
	}
	if found {
		if !shouldSyncStudyCopy(copyRecord, record) {
			return nil
		}
		return saveStudyFromTopperRecordAsCopy(ctx, store, record, copyRecord.ID, copyRecord)
	}

	copyRecord, err = store.GetStudyCopy(ctx, record.ID)
	if err == nil {
		if !shouldSyncStudyCopy(copyRecord, record) {
			return nil
		}
		return saveStudyFromTopperRecordAsCopy(ctx, store, record, record.ID, copyRecord)
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return err
	}
	return saveStudyFromTopperRecord(ctx, store, record)
}

func (h *Handler) syncStudyCopyFromMatchingTopper(
	ctx context.Context,
	store studyStore,
	copyRecord storage.StudyCopyRecord,
	force bool,
) (bool, error) {
	topperStore, ok := h.runtime.Store().(studyTopperStore)
	if !ok || strings.TrimSpace(copyRecord.ID) == "" {
		return false, nil
	}
	record, found, err := findTopperReviewForStudyCopy(ctx, topperStore, copyRecord)
	if err != nil || !found {
		return false, err
	}
	if !force && !shouldSyncStudyCopy(copyRecord, record) {
		return false, nil
	}
	return true, saveStudyFromTopperRecordAsCopy(ctx, store, record, copyRecord.ID, copyRecord)
}

func findTopperReviewForStudyCopy(
	ctx context.Context,
	store studyTopperStore,
	copyRecord storage.StudyCopyRecord,
) (storage.TopperReviewRecord, bool, error) {
	query := studyCopyPDFName(copyRecord)
	if query == "" {
		return storage.TopperReviewRecord{}, false, nil
	}
	summaries, err := store.ListTopperReviews(ctx, storage.TopperReviewListOptions{Query: query, Limit: 200})
	if err != nil {
		return storage.TopperReviewRecord{}, false, err
	}
	best, found := chooseTopperReviewSummary(summaries, copyRecord)
	if !found {
		return storage.TopperReviewRecord{}, false, nil
	}
	record, err := store.GetTopperReview(ctx, best.ID)
	if err != nil {
		return storage.TopperReviewRecord{}, false, err
	}
	return record, true, nil
}

func findStudyCopyForTopper(
	ctx context.Context,
	store studyStore,
	record storage.TopperReviewRecord,
) (storage.StudyCopyRecord, bool, error) {
	query := topperRecordPDFName(record)
	if query == "" {
		return storage.StudyCopyRecord{}, false, nil
	}
	copies, err := store.ListStudyCopies(ctx, storage.StudyCopyListOptions{Query: query, Limit: 200})
	if err != nil {
		return storage.StudyCopyRecord{}, false, err
	}
	var best storage.StudyCopyRecord
	bestRank := 0
	found := false
	for _, copyRecord := range copies {
		rank := studyTopperMatchRank(copyRecord, record)
		if rank == 0 || copyRecord.ID == record.ID {
			continue
		}
		if !found || rank > bestRank || (rank == bestRank && copyRecord.UpdatedAt.After(best.UpdatedAt)) {
			best = copyRecord
			bestRank = rank
			found = true
		}
	}
	return best, found, nil
}

func chooseTopperReviewSummary(
	records []storage.TopperReviewRecord,
	copyRecord storage.StudyCopyRecord,
) (storage.TopperReviewRecord, bool) {
	var best storage.TopperReviewRecord
	bestRank := 0
	found := false
	for _, record := range records {
		rank := topperStudyMatchRank(record, copyRecord)
		if rank == 0 {
			continue
		}
		if !found || rank > bestRank || (rank == bestRank && record.UpdatedAt.After(best.UpdatedAt)) {
			best = record
			bestRank = rank
			found = true
		}
	}
	return best, found
}

func topperStudyMatchRank(record storage.TopperReviewRecord, copyRecord storage.StudyCopyRecord) int {
	rank := 0
	sourcePath := strings.ToLower(strings.TrimSpace(copyRecord.SourcePath))
	recordSourcePath := strings.ToLower(strings.TrimSpace(record.SourcePath))
	if sourcePath != "" && recordSourcePath != "" && recordSourcePath != sourcePath {
		return 0
	}
	if sourcePath != "" && recordSourcePath == sourcePath {
		rank += 8
	}
	copyName := strings.ToLower(studyCopyPDFName(copyRecord))
	recordName := strings.ToLower(firstString(record.PDFName, filepath.Base(record.SourcePath)))
	if copyName != "" && recordName == copyName {
		rank += 4
	}
	if strings.EqualFold(record.Status, "ready") {
		rank += 2
	}
	return rank
}

func studyTopperMatchRank(copyRecord storage.StudyCopyRecord, record storage.TopperReviewRecord) int {
	rank := 0
	sourcePath := strings.ToLower(strings.TrimSpace(copyRecord.SourcePath))
	recordSourcePath := strings.ToLower(strings.TrimSpace(record.SourcePath))
	if sourcePath != "" && recordSourcePath != "" && recordSourcePath != sourcePath {
		return 0
	}
	if sourcePath != "" && recordSourcePath == sourcePath {
		rank += 8
	}
	copyName := strings.ToLower(studyCopyPDFName(copyRecord))
	recordName := strings.ToLower(topperRecordPDFName(record))
	if copyName != "" && recordName == copyName {
		rank += 4
	}
	return rank
}

func topperRecordPDFName(record storage.TopperReviewRecord) string {
	if strings.TrimSpace(record.PDFName) != "" {
		return strings.TrimSpace(record.PDFName)
	}
	if strings.TrimSpace(record.SourcePath) == "" {
		return ""
	}
	name := filepath.Base(record.SourcePath)
	if name == "." {
		return ""
	}
	return name
}

func studyCopyPDFName(copyRecord storage.StudyCopyRecord) string {
	if strings.TrimSpace(copyRecord.PDFName) != "" {
		return strings.TrimSpace(copyRecord.PDFName)
	}
	if strings.TrimSpace(copyRecord.SourcePath) == "" {
		return ""
	}
	name := filepath.Base(copyRecord.SourcePath)
	if name == "." {
		return ""
	}
	return name
}

func shouldSyncStudyCopy(copyRecord storage.StudyCopyRecord, record storage.TopperReviewRecord) bool {
	if copyRecord.PageCount == 0 && record.PageCount > 0 {
		return true
	}
	if copyRecord.QuestionCount == 0 && record.QuestionCount > 0 {
		return true
	}
	if strings.EqualFold(copyRecord.Status, "imported") && !strings.EqualFold(record.Status, "imported") {
		return true
	}
	return record.UpdatedAt.After(copyRecord.UpdatedAt)
}

func saveStudyFromTopperRecordAsCopy(
	ctx context.Context,
	store studyStore,
	record storage.TopperReviewRecord,
	copyID string,
	existing storage.StudyCopyRecord,
) error {
	var review analyze.Response
	if err := json.Unmarshal([]byte(record.ReviewJSON), &review); err != nil {
		return fmt.Errorf("parse topper review %s: %w", record.ID, err)
	}
	copyID = firstString(copyID, record.ID)
	createdAt := firstTime(existing.CreatedAt, record.CreatedAt)
	copyRecord := storage.StudyCopyRecord{
		ID:             copyID,
		SourcePath:     firstString(existing.SourcePath, record.SourcePath),
		SourceHash:     firstString(existing.SourceHash, record.ID),
		PDFName:        firstString(existing.PDFName, record.PDFName, review.PDFName),
		CandidateName:  existing.CandidateName,
		RollNo:         existing.RollNo,
		Email:          existing.Email,
		TestCode:       existing.TestCode,
		Paper:          existing.Paper,
		CopyDate:       existing.CopyDate,
		PageCount:      len(review.Pages),
		QuestionCount:  len(review.Questions),
		UnclearCount:   record.UnclearCount,
		Status:         firstString(record.Status, existing.Status, "ready"),
		RenderStatus:   "ready",
		OCRStatus:      statusFromCount(len(review.Pages)),
		QuestionStatus: statusFromCount(len(review.Questions)),
		AnalysisStatus: studyAnalysisStatus(review),
		ReportStatus:   statusFromText(review.Report),
		CreatedAt:      createdAt,
	}
	pages := studyPagesFromTopper(copyID, record.CreatedAt, review.Pages)
	questions, analyses := studyQuestionsAndAnalysesFromTopper(copyID, record, review)
	return store.ReplaceStudyCopyResult(ctx, copyRecord, pages, questions, analyses)
}

func studyPagesFromTopper(
	copyID string,
	createdAt time.Time,
	pages []analyze.Page,
) []storage.StudyPageRecord {
	out := make([]storage.StudyPageRecord, 0, len(pages))
	for _, page := range pages {
		out = append(out, storage.StudyPageRecord{
			CopyID:       copyID,
			PageNumber:   page.Number,
			Name:         page.Name,
			ImagePath:    page.Path,
			ImageURL:     page.ImageURL,
			OCRText:      page.Text,
			RawOCR:       page.Text,
			Status:       "ready",
			UnclearCount: page.UnclearCount,
			Verified:     page.Verified,
			CreatedAt:    createdAt,
		})
	}
	return out
}

func studyQuestionsAndAnalysesFromTopper(
	copyID string,
	record storage.TopperReviewRecord,
	review analyze.Response,
) ([]storage.StudyQuestionRecord, []storage.StudyAnalysisRecord) {
	questions := make([]storage.StudyQuestionRecord, 0, len(review.Questions))
	analyses := make([]storage.StudyAnalysisRecord, 0, len(review.Questions)*4+1)
	for index, question := range review.Questions {
		qid := scopedStudyQuestionID(copyID, question.ID, index+1)
		questions = append(questions, storage.StudyQuestionRecord{
			ID:          qid,
			CopyID:      copyID,
			QuestionNo:  inferQuestionNo(question.Label, index+1),
			Label:       question.Label,
			PromptText:  question.Title,
			AnswerText:  question.AnswerMarkdown,
			SourcePages: question.SourcePages,
			Status:      firstString(question.Status, "ready"),
			CreatedAt:   record.CreatedAt,
		})
		analyses = append(analyses, studyQuestionDimensionAnalyses(copyID, qid, record, question)...)
	}
	if strings.TrimSpace(review.Report) != "" {
		analyses = append(analyses, storage.StudyAnalysisRecord{
			ID:           copyID + "-report",
			CopyID:       copyID,
			ScopeType:    "copy",
			ScopeID:      copyID,
			DimensionKey: "report",
			ProviderID:   record.ProviderID,
			Model:        record.Model,
			ResultJSON:   jsonString(map[string]string{"report": review.Report}),
			CreatedAt:    record.CreatedAt,
		})
	}
	return questions, analyses
}

func studyQuestionDimensionAnalyses(
	copyID string,
	qid string,
	record storage.TopperReviewRecord,
	question analyze.Question,
) []storage.StudyAnalysisRecord {
	if question.Dimensions == nil {
		return nil
	}
	dims := map[string]string{
		"introduction": question.Dimensions.Introduction,
		"outro":        question.Dimensions.Outro,
		"transition":   question.Dimensions.Transition,
		"diagram":      question.Dimensions.Diagram,
		"fact":         question.Dimensions.Fact,
		"fact_usage":   question.Dimensions.FactUsage,
		"custom":       question.Dimensions.Custom,
	}
	out := []storage.StudyAnalysisRecord{}
	for key, value := range dims {
		if strings.TrimSpace(value) == "" {
			continue
		}
		out = append(out, storage.StudyAnalysisRecord{
			ID:           fmt.Sprintf("%s-dim-%s", qid, key),
			CopyID:       copyID,
			ScopeType:    "question",
			ScopeID:      qid,
			DimensionKey: key,
			ProviderID:   record.ProviderID,
			Model:        record.Model,
			ResultJSON:   jsonString(map[string]string{"analysis": value}),
			CreatedAt:    record.CreatedAt,
		})
	}
	return out
}

var unsafeStudyIDCharPattern = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func scopedStudyQuestionID(copyID string, raw string, index int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = fmt.Sprintf("q%d", index)
	}
	if strings.HasPrefix(raw, copyID+"-") {
		return raw
	}
	safe := unsafeStudyIDCharPattern.ReplaceAllString(strings.ToLower(raw), "-")
	safe = strings.Trim(safe, ".-_")
	if safe == "" {
		safe = fmt.Sprintf("q%d", index)
	}
	return copyID + "-" + safe
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func studyAnalysisStatus(review analyze.Response) string {
	if strings.TrimSpace(review.Report) != "" {
		return "ready"
	}
	for _, question := range review.Questions {
		if question.Dimensions != nil {
			return "ready"
		}
	}
	return "pending"
}
