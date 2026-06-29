package studyapi

import (
	"context"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/storage"
	"github.com/bhickta/aicli/internal/workflow/analyze"
)

func TestSaveStudyMetadataUpdatesMetadataWithoutChangingAnswerText(t *testing.T) {
	t.Parallel()

	store := newStudySyncTestStore(t)
	ctx := context.Background()
	copyRecord := storage.StudyCopyRecord{
		ID:             "study-1",
		SourcePath:     "/tmp/forumias-gs2.pdf",
		PDFName:        "forumias-gs2.pdf",
		Status:         "ready",
		OCRStatus:      "ready",
		QuestionStatus: "ready",
	}
	question := storage.StudyQuestionRecord{
		ID:          "study-1-q1",
		CopyID:      copyRecord.ID,
		QuestionNo:  1,
		Label:       "Q.1",
		PromptText:  "Discuss federalism.",
		AnswerText:  "Original answer text must stay untouched.",
		SourcePages: []int{1},
		Status:      "ready",
	}
	if err := store.SaveStudyCopy(ctx, copyRecord); err != nil {
		t.Fatalf("SaveStudyCopy() error = %v", err)
	}
	if err := store.SaveStudyQuestion(ctx, question); err != nil {
		t.Fatalf("SaveStudyQuestion() error = %v", err)
	}

	payload := studyMetadataResponse{
		Metadata: analyze.CopyMetadata{
			SuggestedPDFName:  "A Topper - GS2 - ForumIAS.pdf",
			TopperName:        "A Topper",
			Paper:             "GS2",
			CoachingInstitute: "ForumIAS",
			Tags:              []string{"GS2", "Polity"},
		},
		Questions: []studyMetadataQuestion{{
			ID:    question.ID,
			Label: question.Label,
			Metadata: analyze.QuestionMetadata{
				Subject:   "Polity",
				Topic:     "Federalism",
				Paper:     "GS2",
				Marks:     10,
				WordLimit: 150,
			},
		}},
	}
	if err := saveStudyMetadata(ctx, store, copyRecord, []storage.StudyQuestionRecord{question}, payload); err != nil {
		t.Fatalf("saveStudyMetadata() error = %v", err)
	}

	updatedCopy, err := store.GetStudyCopy(ctx, copyRecord.ID)
	if err != nil {
		t.Fatalf("GetStudyCopy() error = %v", err)
	}
	if updatedCopy.PDFName != "A Topper - GS2 - ForumIAS.pdf" || updatedCopy.Paper != "GS2" {
		t.Fatalf("copy = %#v, want metadata-backed name and paper", updatedCopy)
	}
	if !strings.Contains(updatedCopy.MetadataJSON, "ForumIAS") || !strings.Contains(updatedCopy.MetadataJSON, "Federalism") {
		t.Fatalf("metadata_json = %q, want copy and question metadata", updatedCopy.MetadataJSON)
	}

	questions, err := store.ListStudyQuestions(ctx, copyRecord.ID)
	if err != nil {
		t.Fatalf("ListStudyQuestions() error = %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("questions = %#v, want one question", questions)
	}
	if questions[0].AnswerText != question.AnswerText {
		t.Fatalf("AnswerText = %q, want unchanged answer", questions[0].AnswerText)
	}
	if questions[0].Marks != 10 || questions[0].WordLimit != 150 {
		t.Fatalf("question = %#v, want marks and word limit from metadata", questions[0])
	}
	if !strings.Contains(questions[0].MetadataJSON, "Federalism") {
		t.Fatalf("question metadata_json = %q, want topic metadata", questions[0].MetadataJSON)
	}
}
