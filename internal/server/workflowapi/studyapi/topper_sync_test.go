package studyapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

func TestSaveStudyFromTopperRecordAsCopyPersistsSelectedStudyID(t *testing.T) {
	t.Parallel()

	store := newStudySyncTestStore(t)
	ctx := context.Background()
	existing := storage.StudyCopyRecord{
		ID:            "study-1",
		SourcePath:    "/tmp/testing.pdf",
		SourceHash:    "study-hash",
		PDFName:       "testing.pdf",
		CandidateName: "Sample Topper",
		Status:        "imported",
		CreatedAt:     time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC),
	}
	if err := store.SaveStudyCopy(ctx, existing); err != nil {
		t.Fatalf("SaveStudyCopy() error = %v", err)
	}

	record := topperReviewRecordForStudySyncTest(t)
	if err := saveStudyFromTopperRecordAsCopy(ctx, store, record, existing.ID, existing); err != nil {
		t.Fatalf("saveStudyFromTopperRecordAsCopy() error = %v", err)
	}

	copyRecord, err := store.GetStudyCopy(ctx, existing.ID)
	if err != nil {
		t.Fatalf("GetStudyCopy() error = %v", err)
	}
	if copyRecord.ID != existing.ID || copyRecord.CandidateName != existing.CandidateName {
		t.Fatalf("copy = %#v, want selected study id and preserved metadata", copyRecord)
	}
	if copyRecord.QuestionCount != 1 || copyRecord.Status != "ready" {
		t.Fatalf("copy = %#v, want ready review counts", copyRecord)
	}

	questions, err := store.ListStudyQuestions(ctx, existing.ID)
	if err != nil {
		t.Fatalf("ListStudyQuestions() error = %v", err)
	}
	if len(questions) != 1 || questions[0].ID != "study-1-q1" || questions[0].CopyID != existing.ID {
		t.Fatalf("questions = %#v, want selected-copy scoped question", questions)
	}
	if _, err := store.GetStudyCopy(ctx, record.ID); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetStudyCopy(topper id) error = %v, want ErrNotFound", err)
	}
}

func TestSaveStudyFromTopperRecordAsCopyReplacesStaleQuestions(t *testing.T) {
	t.Parallel()

	store := newStudySyncTestStore(t)
	ctx := context.Background()
	existing := storage.StudyCopyRecord{ID: "study-1", SourcePath: "/tmp/testing.pdf", PDFName: "testing.pdf"}
	first := topperReviewRecordForStudySyncTest(t)
	if err := saveStudyFromTopperRecordAsCopy(ctx, store, first, existing.ID, existing); err != nil {
		t.Fatalf("first save error = %v", err)
	}

	secondReview := analyze.Response{
		Kind:     "topper_copy_review",
		ReviewID: "topper-1",
		PDFName:  "testing.pdf",
		Pages:    []analyze.Page{{Number: 1, Name: "page-1", Text: "no questions now"}},
		Report:   "updated report",
	}
	data, err := json.Marshal(secondReview)
	if err != nil {
		t.Fatal(err)
	}
	second := first
	second.PageCount = 1
	second.QuestionCount = 0
	second.ReviewJSON = string(data)
	second.UpdatedAt = first.UpdatedAt.Add(time.Minute)
	if err := saveStudyFromTopperRecordAsCopy(ctx, store, second, existing.ID, existing); err != nil {
		t.Fatalf("second save error = %v", err)
	}

	questions, err := store.ListStudyQuestions(ctx, existing.ID)
	if err != nil {
		t.Fatalf("ListStudyQuestions() error = %v", err)
	}
	if len(questions) != 0 {
		t.Fatalf("questions = %#v, want stale questions removed", questions)
	}
	copyRecord, err := store.GetStudyCopy(ctx, existing.ID)
	if err != nil {
		t.Fatalf("GetStudyCopy() error = %v", err)
	}
	if copyRecord.QuestionCount != 0 {
		t.Fatalf("QuestionCount = %d, want 0", copyRecord.QuestionCount)
	}
}

func newStudySyncTestStore(t *testing.T) *storage.SQLiteStore {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := storage.NewSQLiteStore(db)
	if err := store.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return store
}

func topperReviewRecordForStudySyncTest(t *testing.T) storage.TopperReviewRecord {
	t.Helper()

	review := analyze.Response{
		Kind:     "topper_copy_review",
		ReviewID: "topper-1",
		PDFName:  "testing.pdf",
		Pages: []analyze.Page{{
			Number: 1,
			Name:   "page-1",
			Text:   "Q.1 answer text",
		}},
		Questions: []analyze.Question{{
			ID:             "q1",
			Label:          "Q.1",
			Title:          "Discuss.",
			AnswerMarkdown: "Answer text",
			SourcePages:    []int{1},
			Status:         "ready",
		}},
		Report: "Copy report",
	}
	data, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	return storage.TopperReviewRecord{
		ID:            review.ReviewID,
		PDFName:       review.PDFName,
		SourcePath:    "/tmp/testing.pdf",
		PageCount:     len(review.Pages),
		QuestionCount: len(review.Questions),
		Status:        "ready",
		ReviewJSON:    string(data),
		CreatedAt:     time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 6, 29, 10, 1, 0, 0, time.UTC),
	}
}
