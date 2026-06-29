package analyze

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct {
	args []string
}

func (r *fakeRunner) CombinedOutput(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	prefix := args[len(args)-1]
	return []byte("ok"), os.WriteFile(prefix+"-1.jpg", []byte("image"), 0o600)
}

type fakeProvider struct {
	id            string
	visionContent string
	visionPrompt  string
	visionCalls   int
	chatPrompt    string
	chatPrompts   []string
	chatResponses []string
	chatErr       error
}

type progressEvent struct {
	stage     string
	completed int
	total     int
	label     string
}

func (p *fakeProvider) ID() string {
	if p.id != "" {
		return p.id
	}
	return "fake"
}
func (fakeProvider) Health(context.Context) error {
	return nil
}
func (fakeProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (p *fakeProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if len(req.Messages) > 0 {
		p.chatPrompt = req.Messages[0].Content
		p.chatPrompts = append(p.chatPrompts, req.Messages[0].Content)
	}
	if p.chatErr != nil {
		return provider.ChatResponse{}, p.chatErr
	}
	if len(p.chatResponses) > 0 {
		content := p.chatResponses[0]
		p.chatResponses = p.chatResponses[1:]
		return provider.ChatResponse{Content: content}, nil
	}
	return provider.ChatResponse{Content: "report"}, nil
}
func (fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (p *fakeProvider) Vision(_ context.Context, req provider.VisionRequest) (provider.ChatResponse, error) {
	p.visionPrompt = req.Prompt
	p.visionCalls++
	if p.visionContent != "" {
		return provider.ChatResponse{Content: p.visionContent}, nil
	}
	return provider.ChatResponse{Content: "page text"}, nil
}

func TestRunAnalyzePipeline(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	fp := &fakeProvider{}
	progressStages := []string{}
	res, err := New(config.ToolConfig{PDFToPPM: "pdftoppm"}, runner, fp).RunWithProgress(
		context.Background(),
		Request{Path: pdf, Model: "model"},
		func(stage string, completed int, total int, label string) {
			if total <= 0 {
				t.Fatalf("progress total = %d, want positive", total)
			}
			progressStages = append(progressStages, stage)
		},
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(res.Pages) != 1 || res.Report != "report" {
		t.Fatalf("Response = %#v, want one page and report", res)
	}
	if res.Kind != "topper_copy_review" || len(res.Questions) != 1 {
		t.Fatalf("Response = %#v, want review kind and fallback question", res)
	}
	if len(progressStages) == 0 || progressStages[len(progressStages)-1] != "topper copy review ready" {
		t.Fatalf("progress stages = %#v, want final ready stage", progressStages)
	}
	if !hasArgPair(runner.args, "-r", "300") {
		t.Fatalf("pdftoppm args = %#v, want default 300 DPI", runner.args)
	}
	for _, want := range []string{"UPSC answer-copy", "diagrams", "marks", "[unclear]"} {
		if !strings.Contains(fp.visionPrompt, want) {
			t.Fatalf("vision prompt missing %q:\n%s", want, fp.visionPrompt)
		}
	}
	for _, want := range []string{"Answer-Wise Analysis", "Reusable Patterns", "Do not invent official model answers", "page text"} {
		if !strings.Contains(fp.chatPrompt, want) {
			t.Fatalf("chat prompt missing %q:\n%s", want, fp.chatPrompt)
		}
	}
}

func TestRunAnalyzeSavesOCRCheckpointBeforeQuestionSplitFailure(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	splitter := &fakeProvider{id: "splitter", chatErr: errors.New("split unavailable")}
	var checkpoint Response
	_, err := New(
		config.ToolConfig{PDFToPPM: "pdftoppm"},
		&fakeRunner{},
		&fakeProvider{visionContent: "saved OCR"},
		WithQuestionProvider(splitter),
		WithOCRCheckpoint(func(review Response) error {
			checkpoint = review
			return nil
		}),
	).Run(context.Background(), Request{
		Path:          pdf,
		Model:         "model",
		QuestionSplit: true,
	})
	if err == nil {
		t.Fatal("Run() error = nil, want question split failure")
	}
	if len(checkpoint.Pages) != 1 || checkpoint.Pages[0].Text != "saved OCR" {
		t.Fatalf("checkpoint = %#v, want saved OCR page", checkpoint)
	}
	if !strings.Contains(checkpoint.Report, "OCR checkpoint saved") {
		t.Fatalf("checkpoint report = %q, want OCR checkpoint marker", checkpoint.Report)
	}
}

func TestRunAnalyzeReusesSavedOCRPages(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	ocrProvider := &fakeProvider{}
	res, err := New(
		config.ToolConfig{PDFToPPM: "pdftoppm"},
		runner,
		ocrProvider,
	).Run(context.Background(), Request{
		Path:     "cached.pdf",
		Model:    "model",
		ReviewID: "cached-review",
		OCRPages: []Page{{Number: 1, Name: "page-1", Text: "saved page text"}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(runner.args) != 0 || ocrProvider.visionCalls != 0 {
		t.Fatalf("render args = %#v, vision calls = %d; want cached OCR reuse", runner.args, ocrProvider.visionCalls)
	}
	if res.ReviewID != "cached-review" || len(res.Pages) != 1 || res.Pages[0].Text != "saved page text" {
		t.Fatalf("response = %#v, want cached review/pages", res)
	}
}

func TestMergeQuestionBlocksAttachesContinuationToPreviousQuestion(t *testing.T) {
	t.Parallel()

	questions := mergeQuestionBlocks([]Question{
		{
			ID:             "page-3-continuation",
			Label:          "Page 3 continuation",
			AnswerMarkdown: "continued answer",
			SourcePages:    []int{3},
			Status:         "detected",
		},
		{
			ID:             "q.1",
			Label:          "Q.1",
			Title:          "Question one",
			AnswerMarkdown: "main answer",
			SourcePages:    []int{2},
			Status:         "detected",
		},
	})
	if len(questions) != 1 {
		t.Fatalf("questions = %#v, want one merged question", questions)
	}
	got := questions[0]
	if got.ID != "q.1" || got.Label != "Q.1" || !strings.Contains(got.AnswerMarkdown, "continued answer") {
		t.Fatalf("merged question = %#v, want continuation attached to Q.1", got)
	}
	if len(got.SourcePages) != 2 || got.SourcePages[0] != 2 || got.SourcePages[1] != 3 {
		t.Fatalf("source pages = %#v, want [2 3]", got.SourcePages)
	}
}

func TestAnswerBearingPagesExcludeCoverAndOCRFailures(t *testing.T) {
	t.Parallel()

	pages := answerBearingPages([]Page{
		{
			Number: 1,
			Text:   "ForumIAS ACADEMY\nName Of Candidate\nINDEX TABLE\nINSTRUCTIONS\nMaximum Marks",
		},
		{
			Number: 2,
			Text:   "> OCR failed for this page: OCR response was empty",
		},
		{
			Number: 3,
			Text:   "Q.2 answer body with useful content",
		},
	})
	if len(pages) != 1 || pages[0].Number != 3 {
		t.Fatalf("answerBearingPages() = %#v, want only page 3", pages)
	}
}

func TestQuestionsForPagesDropsNonAnswerQuestions(t *testing.T) {
	t.Parallel()

	got := questionsForPages([]Question{
		{Label: "Page 1", SourcePages: []int{1}},
		{Label: "Q.2", SourcePages: []int{4, 5}},
	}, []Page{{Number: 4}, {Number: 5}})
	if len(got) != 1 || got[0].Label != "Q.2" {
		t.Fatalf("questionsForPages() = %#v, want only Q.2", got)
	}
}

func TestRunAnalyzeSplitsQuestionsAndWritesArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pdf := filepath.Join(dir, "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	fp := &fakeProvider{chatResponses: []string{
		`{"questions":[{"label":"Q1","title":"Polity","answer_markdown":"answer block","status":"detected"}]}`,
		"final report",
	}}
	res, err := New(
		config.ToolConfig{PDFToPPM: "pdftoppm"},
		runner,
		fp,
		WithArtifactDir(filepath.Join(dir, "artifacts")),
	).Run(context.Background(), Request{Path: pdf, Model: "model", QuestionSplit: true, QuestionWorkers: 8})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Report != "final report" || len(res.Questions) != 1 || res.Questions[0].Label != "Q1" {
		t.Fatalf("Response = %#v, want split question and final report", res)
	}
	if len(fp.chatPrompts) != 2 {
		t.Fatalf("chat calls = %d, want split + report", len(fp.chatPrompts))
	}
	if !strings.Contains(fp.chatPrompts[0], "Split this OCR") {
		t.Fatalf("first chat prompt = %q, want question split", fp.chatPrompts[0])
	}
	if res.Pages[0].ImageURL == "" {
		t.Fatalf("page image url is empty: %#v", res.Pages[0])
	}
	if _, err := os.Stat(filepath.Join(dir, "artifacts", "topper-copy", res.ReviewID, "review.json")); err != nil {
		t.Fatalf("review artifact not written: %v", err)
	}
}

func TestRunAnalyzeQuestionSplitFallsBackOnEmptyPageResponse(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	fp := &fakeProvider{chatResponses: []string{"", "final report"}}
	res, err := New(config.ToolConfig{PDFToPPM: "pdftoppm"}, &fakeRunner{}, fp).Run(
		context.Background(),
		Request{Path: pdf, Model: "model", QuestionSplit: true},
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Report != "final report" {
		t.Fatalf("report = %q, want final report", res.Report)
	}
	if len(res.Questions) != 1 {
		t.Fatalf("questions = %#v, want page fallback question", res.Questions)
	}
	question := res.Questions[0]
	if question.Label != "Page 1" || question.Status != "needs review" || question.AnswerMarkdown != "page text" {
		t.Fatalf("question = %#v, want fallback page block", question)
	}
}

func TestRunAnalyzeProgressUsesPhaseUnits(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	events := []progressEvent{}
	_, err := New(config.ToolConfig{PDFToPPM: "pdftoppm"}, &fakeRunner{}, &fakeProvider{}).RunWithProgress(
		context.Background(),
		Request{Path: pdf, Model: "model"},
		func(stage string, completed int, total int, label string) {
			events = append(events, progressEvent{stage: stage, completed: completed, total: total, label: label})
		},
	)
	if err != nil {
		t.Fatalf("RunWithProgress() error = %v", err)
	}
	if !hasProgressEvent(events, "OCR pages with", 1, 1, "page") {
		t.Fatalf("progress events = %#v, want OCR phase reported as 1/1 page", events)
	}
	if !hasProgressEvent(events, "topper copy review ready", 3, 3, "step") {
		t.Fatalf("progress events = %#v, want workflow completion reported as 3/3 steps", events)
	}
}

func TestRunAnalyzeUsesSeparateStepProviders(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	ocrProvider := &fakeProvider{id: "ocr", visionContent: "ocr page text"}
	questionProvider := &fakeProvider{id: "question", chatResponses: []string{
		`{"questions":[{"label":"Q1","answer_markdown":"question answer","status":"detected"}]}`,
	}}
	reportProvider := &fakeProvider{id: "report", chatResponses: []string{"report text"}}
	res, err := New(
		config.ToolConfig{PDFToPPM: "pdftoppm"},
		&fakeRunner{},
		ocrProvider,
		WithQuestionProvider(questionProvider),
		WithReportProvider(reportProvider),
	).Run(context.Background(), Request{
		Path:          pdf,
		OCRModel:      "vision-model",
		QuestionModel: "split-model",
		ReportModel:   "report-model",
		QuestionSplit: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Report != "report text" || len(res.Questions) != 1 || res.Questions[0].AnswerMarkdown != "question answer" {
		t.Fatalf("Response = %#v, want split question and report", res)
	}
	if ocrProvider.visionPrompt == "" {
		t.Fatal("ocr provider was not used")
	}
	if !strings.Contains(questionProvider.chatPrompt, "Split this OCR") {
		t.Fatalf("question provider prompt = %q, want question split prompt", questionProvider.chatPrompt)
	}
	if !strings.Contains(reportProvider.chatPrompt, "Answer-Wise Analysis") {
		t.Fatalf("report provider prompt = %q, want report prompt", reportProvider.chatPrompt)
	}
}

func hasProgressEvent(events []progressEvent, prefix string, completed int, total int, label string) bool {
	for _, event := range events {
		if strings.HasPrefix(event.stage, prefix) && event.completed == completed && event.total == total && event.label == label {
			return true
		}
	}
	return false
}

func TestParseQuestionSplitAcceptsWrappedJSONAndAnswerAlias(t *testing.T) {
	t.Parallel()

	content := "Here is the split:\n```json\n{\"questions\":[{\"question\":\"Q.1\",\"title\":\"Women in ancient India\",\"answer\":\"full answer text\",\"status\":\"detected\"}]}\n```"
	questions, err := parseQuestionSplit(content, 3)
	if err != nil {
		t.Fatalf("parseQuestionSplit() error = %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("questions = %#v, want one question", questions)
	}
	if questions[0].Label != "Q.1" || questions[0].AnswerMarkdown != "full answer text" || questions[0].SourcePages[0] != 3 {
		t.Fatalf("question = %#v, want parsed alias fields", questions[0])
	}
}

func TestParseQuestionSplitRejectsEmptyQuestionBlocks(t *testing.T) {
	t.Parallel()

	_, err := parseQuestionSplit(`{"questions":[]}`, 1)
	if err == nil {
		t.Fatal("parseQuestionSplit() error = nil, want empty-block error")
	}
}

func TestParseDirectPDFReview(t *testing.T) {
	t.Parallel()

	content := "```json\n{\"pages\":[{\"number\":1,\"name\":\"page-1\",\"text\":\"ocr text\",\"unclear_count\":1}],\"questions\":[{\"label\":\"Q.1\",\"title\":\"History\",\"answer_markdown\":\"answer text\",\"source_pages\":[1],\"status\":\"detected\"}],\"report\":\"final report\"}\n```"
	review, err := parseDirectPDFReview(content, "review-1", "copy.pdf")
	if err != nil {
		t.Fatalf("parseDirectPDFReview() error = %v", err)
	}
	if review.Kind != "topper_copy_review" || review.ReviewID != "review-1" || review.PDFName != "copy.pdf" {
		t.Fatalf("review metadata = %#v", review)
	}
	if len(review.Pages) != 1 || review.Pages[0].Text != "ocr text" || review.Pages[0].UnclearCount != 1 {
		t.Fatalf("pages = %#v", review.Pages)
	}
	if len(review.Questions) != 1 || review.Questions[0].AnswerMarkdown != "answer text" || review.Report != "final report" {
		t.Fatalf("review = %#v, want question and report", review)
	}
}

func TestRunAnalyzeHonorsExplicitDPI(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	_, err := New(config.ToolConfig{PDFToPPM: "pdftoppm"}, runner, &fakeProvider{}).Run(
		context.Background(),
		Request{Path: pdf, Model: "model", DPI: 220},
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !hasArgPair(runner.args, "-r", "220") {
		t.Fatalf("pdftoppm args = %#v, want explicit 220 DPI", runner.args)
	}
}

func TestEffectiveQuestionWorkersHonorsExplicitWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		workers int
		total   int
		want    int
	}{
		{name: "uses explicit worker count", workers: 48, total: 48, want: 48},
		{name: "caps explicit workers by available pages", workers: 48, total: 8, want: 8},
		{name: "single page needs one worker", workers: 48, total: 1, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveQuestionWorkers(tt.workers, tt.total); got != tt.want {
				t.Fatalf("EffectiveQuestionWorkers(%d, %d) = %d, want %d", tt.workers, tt.total, got, tt.want)
			}
		})
	}
}

func hasArgPair(args []string, key string, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
