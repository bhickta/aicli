package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

func TestDecodeBoundaryBenchmarkSuiteRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "unknown field", input: `{"version":1,"cases":[],"repair":true}`},
		{name: "trailing content", input: `{"version":1,"cases":[]} accepted`},
		{name: "unsupported version", input: `{"version":2,"cases":[{"id":"case","pages":[{"number":1,"text":""}],"expected_questions":[{"label":"","source_pages":[1]}]}]}`},
		{name: "empty cases", input: `{"version":1,"cases":[]}`},
		{name: "duplicate case id", input: `{"version":1,"cases":[{"id":"case","pages":[{"number":1,"text":""}],"expected_questions":[{"label":"","source_pages":[1]}]},{"id":"case","pages":[{"number":2,"text":""}],"expected_questions":[{"label":"","source_pages":[2]}]}]}`},
		{name: "padded case id", input: `{"version":1,"cases":[{"id":" case ","pages":[{"number":1,"text":""}],"expected_questions":[{"label":"","source_pages":[1]}]}]}`},
		{name: "duplicate page", input: `{"version":1,"cases":[{"id":"case","pages":[{"number":1,"text":"a"},{"number":1,"text":"b"}],"expected_questions":[{"label":"","source_pages":[1]}]}]}`},
		{name: "coverage gap", input: `{"version":1,"cases":[{"id":"case","pages":[{"number":1,"text":"a"},{"number":2,"text":"b"}],"expected_questions":[{"label":"","source_pages":[1]}]}]}`},
		{name: "noncontiguous group", input: `{"version":1,"cases":[{"id":"case","pages":[{"number":1,"text":"a"},{"number":2,"text":"b"},{"number":3,"text":"c"}],"expected_questions":[{"label":"Q1","source_pages":[1,3]},{"label":"Q2","source_pages":[2]}]}]}`},
		{name: "invalid expected status", input: `{"version":1,"cases":[{"id":"case","pages":[{"number":1,"text":""}],"expected_questions":[{"label":"","source_pages":[1],"status":"ready"}]}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := DecodeBoundaryBenchmarkSuite(strings.NewReader(tt.input)); err == nil {
				t.Fatalf("DecodeBoundaryBenchmarkSuite() error = nil for %s", tt.input)
			}
		})
	}
}

func TestRunBoundaryBenchmarkScoresGroundingAndStability(t *testing.T) {
	t.Parallel()

	suite := BoundaryBenchmarkSuite{
		Version: BoundaryBenchmarkVersion,
		Cases: []BoundaryBenchmarkCase{
			{
				ID: "two-answers",
				Pages: []BoundaryBenchmarkPage{
					{Number: 1, Text: "Q1 first page"},
					{Number: 2, Text: "essay continuation"},
					{Number: 3, Text: "Q2 second answer"},
				},
				ExpectedQuestions: []BoundaryBenchmarkExpectedQuestion{
					{Label: "Q1", SourcePages: []int{1, 2}, Status: "detected"},
					{Label: "Q2", SourcePages: []int{3}, Status: "detected"},
				},
			},
		},
	}
	provider := &fakeProvider{chatResponses: []string{
		`{"decisions":[{"page_number":1,"boundary":"new_answer","boundary_confidence":0.95,"visible_label":"Q1","label_evidence":"Q1","reason":"visible"},{"page_number":2,"boundary":"continuation","boundary_confidence":0.9,"visible_label":"","label_evidence":"","reason":"continues"},{"page_number":3,"boundary":"new_answer","boundary_confidence":0.95,"visible_label":"Q2","label_evidence":"Q2","reason":"visible"}]}`,
		`{"decisions":[{"page_number":1,"boundary":"new_answer","boundary_confidence":0.8,"visible_label":"","label_evidence":"","reason":"start"},{"page_number":2,"boundary":"continuation","boundary_confidence":0.8,"visible_label":"","label_evidence":"","reason":"continues"},{"page_number":3,"boundary":"new_answer","boundary_confidence":0.8,"visible_label":"","label_evidence":"","reason":"start"}]}`,
	}}
	service := New(config.ToolConfig{}, nil, provider, WithQuestionProvider(provider))

	report, err := service.RunBoundaryBenchmark(context.Background(), "local-model", suite, 2)
	if err != nil {
		t.Fatalf("RunBoundaryBenchmark() error = %v", err)
	}
	if report.Summary.AttemptedRuns != 2 || report.Summary.SchemaValidRuns != 2 || report.Summary.ExactGroupingRuns != 2 {
		t.Fatalf("summary = %#v, want two valid exact-grouping runs", report.Summary)
	}
	if report.Summary.LabelMatches != 2 || report.Summary.LabelChecks != 4 || report.Summary.LabelAccuracy == nil || *report.Summary.LabelAccuracy != 0.5 {
		t.Fatalf("label summary = %#v, want 2/4", report.Summary)
	}
	if report.Summary.StatusMatches != 2 || report.Summary.StatusChecks != 4 || report.Summary.StatusAccuracy == nil || *report.Summary.StatusAccuracy != 0.5 {
		t.Fatalf("status summary = %#v, want 2/4", report.Summary)
	}
	if report.Summary.StableCases != 0 || report.Summary.StabilityRate == nil || *report.Summary.StabilityRate != 0 {
		t.Fatalf("stability summary = %#v, want unstable case", report.Summary)
	}
	if len(report.Runs) != 2 || report.Runs[1].ObservedQuestions[0].Label != "Page 1 block" {
		t.Fatalf("runs = %#v, want ungrounded labels preserved as review blocks", report.Runs)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "essay continuation") {
		t.Fatal("benchmark report leaked source OCR text")
	}
}

func TestRunBoundaryBenchmarkSeparatesRequestAndSchemaFailures(t *testing.T) {
	t.Parallel()

	suite := onePageBoundaryBenchmarkSuite()
	tests := []struct {
		name          string
		provider      *fakeProvider
		wantErrorKind string
		wantRequests  int
	}{
		{
			name:          "request failure",
			provider:      &fakeProvider{chatErr: errors.New("model unavailable")},
			wantErrorKind: "request",
			wantRequests:  0,
		},
		{
			name:          "schema failure",
			provider:      &fakeProvider{chatResponses: []string{`{"decisions":[{"page_number":1,"boundary":"invented","boundary_confidence":0.9,"visible_label":"Q1","label_evidence":"Q1","reason":"visible"}]}`}},
			wantErrorKind: "schema",
			wantRequests:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := New(config.ToolConfig{}, nil, tt.provider, WithQuestionProvider(tt.provider))
			report, err := service.RunBoundaryBenchmark(context.Background(), "local-model", suite, 1)
			if err != nil {
				t.Fatalf("RunBoundaryBenchmark() error = %v", err)
			}
			if len(report.Runs) != 1 || report.Runs[0].ErrorKind != tt.wantErrorKind {
				t.Fatalf("runs = %#v, want error kind %q", report.Runs, tt.wantErrorKind)
			}
			if report.Summary.SuccessfulRequests != tt.wantRequests || report.Summary.SchemaValidRuns != 0 {
				t.Fatalf("summary = %#v, want %d successful requests and no valid schemas", report.Summary, tt.wantRequests)
			}
		})
	}
}

func onePageBoundaryBenchmarkSuite() BoundaryBenchmarkSuite {
	return BoundaryBenchmarkSuite{
		Version: BoundaryBenchmarkVersion,
		Cases: []BoundaryBenchmarkCase{
			{
				ID:                "one-page",
				Pages:             []BoundaryBenchmarkPage{{Number: 1, Text: "Q1 answer"}},
				ExpectedQuestions: []BoundaryBenchmarkExpectedQuestion{{Label: "Q1", SourcePages: []int{1}}},
			},
		},
	}
}
