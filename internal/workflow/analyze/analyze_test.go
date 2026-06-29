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
	id                string
	visionContent     string
	visionPrompt      string
	visionCalls       int
	documentContent   string
	documentResponses []string
	documentPrompt    string
	documentCalls     int
	documentReason    string
	chatPrompt        string
	chatPrompts       []string
	chatResponses     []string
	chatErr           error
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
func (p *fakeProvider) Document(_ context.Context, req provider.DocumentRequest) (provider.DocumentResponse, error) {
	p.documentPrompt = req.Prompt
	p.documentCalls++
	if len(p.documentResponses) > 0 {
		content := p.documentResponses[0]
		p.documentResponses = p.documentResponses[1:]
		return provider.DocumentResponse{Content: content, FinishReason: p.documentReason}, nil
	}
	return provider.DocumentResponse{Content: p.documentContent, FinishReason: p.documentReason}, nil
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

func TestRunAnalyzeDirectPDFUsesGeminiDocumentOnly(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{
		id: "gemini",
		documentContent: `{
			"metadata":{"suggested_pdf_name":"Sample Topper - GS2.pdf","topper_name":"Sample Topper","paper":"GS2","coaching_institute":"ForumIAS","tags":["polity"]},
			"detected_questions":["Q.1"],
			"pages":[{"number":1,"name":"page-1","text":"source notes","unclear_count":0}],
			"questions":[{"id":"q1","label":"Q.1","title":"Polity","source_pages":[1],"answer_markdown":"answer text","dimensions":{"introduction":"clear intro","fact":"Article 21"},"metadata":{"subject":"Polity","topic":"Fundamental Rights","marks":10,"word_limit":150,"tags":["article 21"]}}],
			"report":"final report"
		}`,
	}
	res, err := New(config.ToolConfig{PDFToPPM: "pdftoppm"}, &fakeRunner{}, provider).Run(
		context.Background(),
		Request{Path: pdf, OCRModel: "gemini-2.5-flash-lite", OCRInputMode: OCRInputModeAuto},
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if provider.documentCalls != 1 || provider.visionCalls != 0 || len(provider.chatPrompts) != 0 {
		t.Fatalf("calls: document=%d vision=%d chat=%d, want document only", provider.documentCalls, provider.visionCalls, len(provider.chatPrompts))
	}
	if res.SourceMode != OCRInputModePDFDirect || res.Report != "final report" || len(res.Questions) != 1 {
		t.Fatalf("response = %#v, want direct PDF review", res)
	}
	if res.Questions[0].Dimensions == nil || res.Questions[0].Dimensions.Introduction != "clear intro" {
		t.Fatalf("dimensions = %#v, want parsed dimensions", res.Questions[0].Dimensions)
	}
	if res.Metadata == nil || res.Metadata.TopperName != "Sample Topper" || res.Metadata.Paper != "GS2" {
		t.Fatalf("metadata = %#v, want parsed copy metadata", res.Metadata)
	}
	if res.Questions[0].Metadata == nil || res.Questions[0].Metadata.Topic != "Fundamental Rights" || res.Questions[0].Metadata.Marks != 10 {
		t.Fatalf("question metadata = %#v, want parsed question metadata", res.Questions[0].Metadata)
	}
	if !strings.Contains(provider.documentPrompt, "Gemini Flash-Lite") || !strings.Contains(provider.documentPrompt, "valid JSON only") {
		t.Fatalf("document prompt = %q, want Gemini-Lite JSON prompt", provider.documentPrompt)
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
		`{"introduction":"good","outro":"fine","transition":"ok","diagram":"none"}`,
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
	if len(fp.chatPrompts) != 3 {
		t.Fatalf("chat calls = %d, want split + dimensions + report", len(fp.chatPrompts))
	}
	if !strings.Contains(fp.chatPrompts[0], "Split this OCR") {
		t.Fatalf("first chat prompt = %q, want question split", fp.chatPrompts[0])
	}
	if !strings.Contains(fp.chatPrompts[1], "Analyze the structural dimensions") {
		t.Fatalf("second chat prompt = %q, want dimensions", fp.chatPrompts[1])
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
		`{"introduction":"good"}`,
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
	if !strings.Contains(questionProvider.chatPrompts[0], "Split this OCR") {
		t.Fatalf("question provider prompt = %q, want question split prompt", questionProvider.chatPrompts[0])
	}
	if !strings.Contains(questionProvider.chatPrompts[1], "Analyze the structural dimensions") {
		t.Fatalf("question provider prompt = %q, want dimensions prompt", questionProvider.chatPrompts[1])
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

func TestParseOneShotPDFManifest(t *testing.T) {
	t.Parallel()

	content := "```json\n{\"metadata\":{\"topper_name\":\"A Topper\",\"paper\":\"GS1\"},\"detected_questions\":[\"Q.1\"],\"pages\":[{\"number\":1,\"name\":\"page-1\",\"text\":\"ocr text\",\"unclear_count\":1}],\"questions\":[{\"label\":\"Q.1\",\"title\":\"History\",\"source_pages\":[1],\"answer_markdown\":\"test answer\",\"dimensions\":{\"fact\":\"good examples\"},\"metadata\":{\"subject\":\"History\",\"topic\":\"Ancient India\",\"marks\":10}}],\"report\":\"test report\"}\n```"
	metadata, pages, questions, report, err := parseOneShotPDFManifest(content, "copy.pdf")
	if err != nil {
		t.Fatalf("parseOneShotPDFManifest() error = %v", err)
	}
	if metadata == nil || metadata.TopperName != "A Topper" || metadata.Paper != "GS1" {
		t.Fatalf("metadata = %#v, want parsed copy metadata", metadata)
	}
	if len(pages) != 1 || pages[0].Text != "ocr text" || pages[0].UnclearCount != 1 {
		t.Fatalf("pages = %#v", pages)
	}
	if len(questions) != 1 || questions[0].Label != "Q.1" || questions[0].Title != "History" || questions[0].SourcePages[0] != 1 || questions[0].AnswerMarkdown != "test answer" {
		t.Fatalf("questions = %#v", questions)
	}
	if questions[0].Dimensions == nil || questions[0].Dimensions.Fact != "good examples" {
		t.Fatalf("dimensions = %#v, want fact dimension", questions[0].Dimensions)
	}
	if questions[0].Metadata == nil || questions[0].Metadata.Topic != "Ancient India" || questions[0].Metadata.Marks != 10 {
		t.Fatalf("question metadata = %#v, want parsed metadata", questions[0].Metadata)
	}
	if report != "test report" {
		t.Fatalf("report = %q", report)
	}
}

func TestParseOneShotPDFManifestRejectsIncompletePayload(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "empty answer", content: `{"pages":[{"number":1}],"questions":[{"label":"Q.1","answer_markdown":""}],"report":"report"}`},
		{name: "empty report", content: `{"pages":[{"number":1}],"questions":[{"label":"Q.1","answer_markdown":"answer"}],"report":""}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, _, _, err := parseOneShotPDFManifest(tt.content, "copy.pdf"); err == nil {
				t.Fatalf("parseOneShotPDFManifest() error = nil, want error for %s", tt.name)
			}
		})
	}
}

func TestParseOneShotPDFManifestRejectsQuestionUnderExtraction(t *testing.T) {
	t.Parallel()

	content := `{
		"detected_questions":["first visible answer block","second visible answer block","third visible answer block"],
		"pages":[
			{"number":3,"text":"first page notes"},
			{"number":5,"text":"second page notes"},
			{"number":7,"text":"third page notes"}
		],
		"questions":[{"label":"first visible answer block","source_pages":[3],"answer_markdown":"answer one"}],
		"report":"report"
	}`
	_, _, _, _, err := parseOneShotPDFManifest(content, "copy.pdf")
	if err == nil || !strings.Contains(err.Error(), "detected 3 question/answer block") {
		t.Fatalf("parseOneShotPDFManifest() error = %v, want model-declared coverage error", err)
	}
}

func TestRunAnalyzeRetriesDirectPDFWhenQuestionCoverageIsIncomplete(t *testing.T) {
	t.Parallel()

	pdf := filepath.Join(t.TempDir(), "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	incomplete := `{
		"detected_questions":["first answer","second answer","third answer"],
		"pages":[
			{"number":3,"text":"Answer to Q.1 starts here."},
			{"number":5,"text":"Answer to Q.2 starts here."},
			{"number":7,"text":"Answer to Q.3 starts here."}
		],
		"questions":[{"label":"Q.1","source_pages":[3],"answer_markdown":"answer one"}],
		"report":"report"
	}`
	complete := `{
		"detected_questions":["first answer","second answer","third answer"],
		"pages":[
			{"number":3,"text":"Answer to Q.1 starts here."},
			{"number":5,"text":"Answer to Q.2 starts here."},
			{"number":7,"text":"Answer to Q.3 starts here."}
		],
		"questions":[
			{"label":"Q.1","source_pages":[3],"answer_markdown":"answer one"},
			{"label":"Q.2","source_pages":[5],"answer_markdown":"answer two"},
			{"label":"Q.3","source_pages":[7],"answer_markdown":"answer three"}
		],
		"report":"report"
	}`
	provider := &fakeProvider{id: "gemini", documentResponses: []string{incomplete, complete}}
	res, err := New(config.ToolConfig{PDFToPPM: "pdftoppm"}, &fakeRunner{}, provider).Run(
		context.Background(),
		Request{Path: pdf, OCRModel: "gemini-flash-lite-latest", OCRInputMode: OCRInputModePDFDirect},
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if provider.documentCalls != 2 || res.APICalls != 2 {
		t.Fatalf("documentCalls=%d apiCalls=%d, want retry accounting", provider.documentCalls, res.APICalls)
	}
	if len(res.Questions) != 3 {
		t.Fatalf("questions = %#v, want three covered questions", res.Questions)
	}
	if !strings.Contains(provider.documentPrompt, "coverage") {
		t.Fatalf("retry prompt = %q, want coverage-focused prompt", provider.documentPrompt)
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
