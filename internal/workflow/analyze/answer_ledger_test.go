package analyze

import (
	"strings"
	"testing"
)

func TestParseAnswerBoundaryLedgerRejectsNonSchemaOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "markdown fence", content: "```json\n{\"decisions\":[]}\n```"},
		{name: "trailing prose", content: "{\"decisions\":[]} accepted"},
		{name: "unknown field", content: "{\"decisions\":[],\"repaired\":true}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := parseAnswerBoundaryLedger(tt.content); err == nil {
				t.Fatalf("parseAnswerBoundaryLedger(%q) error = nil, want rejection", tt.content)
			}
		})
	}
}

func TestQuestionsFromBoundaryLedgerRejectsInvalidCoverage(t *testing.T) {
	t.Parallel()

	pages := []Page{
		{Number: 1, Text: "Q1 answer"},
		{Number: 2, Text: "continuation"},
	}
	validFirst := answerBoundaryDecision{
		PageNumber:         1,
		Boundary:           questionBoundaryNew,
		BoundaryConfidence: 0.95,
		VisibleLabel:       "Q1",
		LabelEvidence:      "Q1",
	}
	tests := []struct {
		name      string
		pages     []Page
		decisions []answerBoundaryDecision
		wantError string
	}{
		{
			name:      "missing page",
			pages:     pages,
			decisions: []answerBoundaryDecision{validFirst},
			wantError: "covered 1 of 2 pages",
		},
		{
			name:      "repeated decision",
			pages:     pages,
			decisions: []answerBoundaryDecision{validFirst, validFirst},
			wantError: "repeated page 1",
		},
		{
			name:  "unknown page",
			pages: pages,
			decisions: []answerBoundaryDecision{
				validFirst,
				{PageNumber: 3, Boundary: questionBoundaryContinuation, BoundaryConfidence: 0.9},
			},
			wantError: "unknown page 3",
		},
		{
			name: "duplicate source page",
			pages: []Page{
				{Number: 1, Text: "first"},
				{Number: 1, Text: "duplicate"},
			},
			decisions: []answerBoundaryDecision{validFirst},
			wantError: "duplicate source page 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := questionsFromBoundaryLedger(
				tt.pages,
				answerBoundaryLedger{Decisions: tt.decisions},
			)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("questionsFromBoundaryLedger() error = %v, want containing %q", err, tt.wantError)
			}
		})
	}
}

func TestQuestionsFromBoundaryLedgerDoesNotMergeUnsafeContinuations(t *testing.T) {
	t.Parallel()

	pages := []Page{
		{Number: 1, Text: "Q1 first answer"},
		{Number: 2, Text: "Q2 second answer"},
	}
	first := answerBoundaryDecision{
		PageNumber:         1,
		Boundary:           questionBoundaryNew,
		BoundaryConfidence: 0.95,
		VisibleLabel:       "Q1",
		LabelEvidence:      "Q1",
	}
	tests := []struct {
		name     string
		decision answerBoundaryDecision
	}{
		{
			name: "grounded label contradicts continuation",
			decision: answerBoundaryDecision{
				PageNumber:         2,
				Boundary:           questionBoundaryContinuation,
				BoundaryConfidence: 0.95,
				VisibleLabel:       "Q2",
				LabelEvidence:      "Q2",
			},
		},
		{
			name: "low confidence continuation",
			decision: answerBoundaryDecision{
				PageNumber:         2,
				Boundary:           questionBoundaryContinuation,
				BoundaryConfidence: 0.4,
			},
		},
		{
			name: "incomplete label evidence",
			decision: answerBoundaryDecision{
				PageNumber:         2,
				Boundary:           questionBoundaryContinuation,
				BoundaryConfidence: 0.95,
				LabelEvidence:      "Q2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			questions, err := questionsFromBoundaryLedger(
				pages,
				answerBoundaryLedger{Decisions: []answerBoundaryDecision{first, tt.decision}},
			)
			if err != nil {
				t.Fatalf("questionsFromBoundaryLedger() error = %v", err)
			}
			if len(questions) != 2 {
				t.Fatalf("questions = %#v, want unsafe continuation kept separate", questions)
			}
			if questions[1].Boundary != questionBoundaryUncertain || questions[1].Status != "needs review" {
				t.Fatalf("second question = %#v, want uncertain needs-review block", questions[1])
			}
		})
	}
}
