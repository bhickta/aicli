package analyze

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
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
	chatModels        []string
	chatResponses     []string
	chatSchemas       []*provider.JSONSchema
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
	p.chatModels = append(p.chatModels, req.Model)
	if len(req.Messages) > 0 {
		p.chatPrompt = req.Messages[0].Content
		p.chatPrompts = append(p.chatPrompts, req.Messages[0].Content)
	}
	p.chatSchemas = append(p.chatSchemas, req.ResponseSchema)
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
	for _, want := range []string{
		"Answer-Wise Analysis",
		"Repeated Winning Patterns",
		"Deliberate-Practice Plan",
		"Do not invent official model answers",
		"page text",
	} {
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
	for _, want := range []string{"Gemini Flash-Lite", "valid JSON only", "demand_fulfilment", "Evidence-based Markdown report"} {
		if !strings.Contains(provider.documentPrompt, want) {
			t.Fatalf("document prompt missing %q:\n%s", want, provider.documentPrompt)
		}
	}
}

func TestMergeQuestionBlocksAttachesContinuationToPreviousQuestion(t *testing.T) {
	t.Parallel()

	questions := mergeQuestionBlocks([]Question{
		{
			ID:                 "page-3-continuation",
			Label:              "Page 3 continuation",
			AnswerMarkdown:     "continued answer",
			SourcePages:        []int{3},
			Status:             "detected",
			Boundary:           questionBoundaryContinuation,
			BoundaryConfidence: 0.95,
		},
		{
			ID:                 "q.1",
			Label:              "Q.1",
			Title:              "Question one",
			AnswerMarkdown:     "main answer",
			SourcePages:        []int{2},
			Status:             "detected",
			Boundary:           questionBoundaryNew,
			BoundaryConfidence: 0.98,
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

func TestMergeQuestionBlocksAttachesGenericAdjacentLabelToSubpart(t *testing.T) {
	t.Parallel()

	questions := mergeQuestionBlocks([]Question{
		{
			ID:                 "q1(a)",
			Label:              "Q1(a)",
			AnswerMarkdown:     "answer starts",
			SourcePages:        []int{2},
			Status:             "detected",
			Boundary:           questionBoundaryNew,
			BoundaryConfidence: 0.98,
		},
		{
			ID:                 "q1",
			Label:              "Q1",
			AnswerMarkdown:     "answer continues without a visible subpart label",
			SourcePages:        []int{3},
			Status:             "detected",
			Boundary:           questionBoundaryContinuation,
			BoundaryConfidence: 0.95,
		},
		{
			ID:                 "1(b)",
			Label:              "1(b)",
			AnswerMarkdown:     "next subpart starts",
			SourcePages:        []int{4},
			Status:             "detected",
			Boundary:           questionBoundaryNew,
			BoundaryConfidence: 0.98,
		},
	})
	if len(questions) != 2 {
		t.Fatalf("questions = %#v, want Q1(a) continuation and separate 1(b)", questions)
	}
	if questions[0].Label != "Q1(a)" || len(questions[0].SourcePages) != 2 || !strings.Contains(questions[0].AnswerMarkdown, "continues") {
		t.Fatalf("first question = %#v, want adjacent Q1 merged into Q1(a)", questions[0])
	}
	if questions[1].Label != "1(b)" {
		t.Fatalf("second question = %#v, want distinct subpart", questions[1])
	}
}

func TestSemanticBoundariesKeepAdjacentSubpartsSeparate(t *testing.T) {
	t.Parallel()

	pagePayloads := []string{
		`{"page_kind":"answer","page_kind_confidence":0.98,"classification_reason":"candidate answer","ocr_confidence":0.95,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"new_answer","boundary_confidence":0.99,"visible_label":"1(a)","title":"","answer_markdown":"attitude answer starts","status":"detected"}]}`,
		`{"page_kind":"answer","page_kind_confidence":0.96,"classification_reason":"candidate answer continuation","ocr_confidence":0.94,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"continuation","boundary_confidence":0.97,"visible_label":"","title":"","answer_markdown":"attitude answer continues","status":"detected"}]}`,
		`{"page_kind":"answer","page_kind_confidence":0.98,"classification_reason":"new labelled candidate answer","ocr_confidence":0.95,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"new_answer","boundary_confidence":0.99,"visible_label":"1(b)","title":"","answer_markdown":"CRISPR answer starts","status":"detected"}]}`,
		`{"page_kind":"answer","page_kind_confidence":0.96,"classification_reason":"candidate answer continuation","ocr_confidence":0.94,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"continuation","boundary_confidence":0.97,"visible_label":"","title":"","answer_markdown":"CRISPR answer continues","status":"detected"}]}`,
	}

	blocks := []Question{}
	for index, payload := range pagePayloads {
		result, err := parsePageQuestionSplit(payload, index+2)
		if err != nil {
			t.Fatalf("parse page %d: %v", index+2, err)
		}
		blocks = append(blocks, result.Questions...)
	}
	questions := mergeQuestionBlocks(blocks)
	if len(questions) != 2 {
		t.Fatalf("questions = %#v, want exactly 1(a) and 1(b)", questions)
	}
	if questions[0].Label != "1(a)" || !slices.Equal(questions[0].SourcePages, []int{2, 3}) {
		t.Fatalf("first question = %#v, want 1(a) on pages 2-3", questions[0])
	}
	if questions[1].Label != "1(b)" || !slices.Equal(questions[1].SourcePages, []int{4, 5}) {
		t.Fatalf("second question = %#v, want 1(b) on pages 4-5", questions[1])
	}
}

func TestSemanticNewAnswersDoNotMergeOnMatchingLabels(t *testing.T) {
	t.Parallel()

	questions := mergeQuestionBlocks([]Question{
		{
			Label:              "Q1",
			AnswerMarkdown:     "first answer",
			SourcePages:        []int{2},
			Status:             "detected",
			Boundary:           questionBoundaryNew,
			BoundaryConfidence: 0.98,
		},
		{
			Label:              "Q1",
			AnswerMarkdown:     "second answer",
			SourcePages:        []int{4},
			Status:             "detected",
			Boundary:           questionBoundaryNew,
			BoundaryConfidence: 0.98,
		},
	})
	if len(questions) != 2 {
		t.Fatalf("questions = %#v, want two semantic new-answer blocks", questions)
	}
	if questions[0].ID == questions[1].ID {
		t.Fatalf("question IDs = %q and %q, want stable unique IDs", questions[0].ID, questions[1].ID)
	}
}

func TestSplitQuestionsSuppliesAllPagesToCopyBoundaryLedger(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{chatResponses: []string{
		`{"page_kind":"answer","page_kind_confidence":0.98,"classification_reason":"new answer","ocr_confidence":0.95,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"new_answer","boundary_confidence":0.98,"visible_label":"1(a)","title":"","answer_markdown":"answer starts","status":"detected"}]}`,
		`{"page_kind":"answer","page_kind_confidence":0.97,"classification_reason":"continuation","ocr_confidence":0.94,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"continuation","boundary_confidence":0.96,"visible_label":"","title":"","answer_markdown":"answer continues","status":"detected"}]}`,
		`{"decisions":[{"page_number":2,"boundary":"new_answer","boundary_confidence":0.98,"visible_label":"1(a)","label_evidence":"1(a)","reason":"visible label"},{"page_number":3,"boundary":"continuation","boundary_confidence":0.96,"visible_label":"","label_evidence":"","reason":"continues previous page"}]}`,
	}}
	service := New(config.ToolConfig{}, &fakeRunner{}, provider)
	_, err := service.splitQuestions(
		context.Background(),
		[]Page{
			{Number: 2, Text: "previous answer text"},
			{Number: 3, Text: "current continuation text"},
		},
		questionSplitOptions{QuestionModel: "local-model", Workers: 1},
		nil,
	)
	if err != nil {
		t.Fatalf("splitQuestions() error = %v", err)
	}
	if len(provider.chatPrompts) != 3 {
		t.Fatalf("chat prompts = %d, want two page classifications and one copy ledger", len(provider.chatPrompts))
	}
	if !slices.Equal(provider.chatModels, []string{"local-model", "local-model", "local-model"}) {
		t.Fatalf("chat models = %#v, want question-model fallback for boundary ledger", provider.chatModels)
	}
	ledgerPrompt := provider.chatPrompts[2]
	if !strings.Contains(ledgerPrompt, "previous answer text") || !strings.Contains(ledgerPrompt, "current continuation text") {
		t.Fatalf("ledger prompt missing full copy boundary context:\n%s", ledgerPrompt)
	}
}

func TestSplitQuestionsUsesCopyLedgerInsteadOfPageFieldGuesses(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{chatResponses: []string{
		`{"page_kind":"answer","page_kind_confidence":0.99,"classification_reason":"answer","ocr_confidence":0.8,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"new_answer","boundary_confidence":1,"visible_label":"","title":"Attitude definition","answer_markdown":"page two model rewrite","status":"needs review"}]}`,
		`{"page_kind":"answer","page_kind_confidence":0.99,"classification_reason":"answer","ocr_confidence":0.8,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"new_answer","boundary_confidence":1,"visible_label":"","title":"Question No.","answer_markdown":"new_answer","status":"needs review"}]}`,
		`{"page_kind":"answer","page_kind_confidence":0.99,"classification_reason":"answer","ocr_confidence":0.8,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"new_answer","boundary_confidence":0.95,"visible_label":"ETHICS ASSOCIATED","title":"1(b)","answer_markdown":"page four model rewrite","status":"needs review"}]}`,
		`{"page_kind":"answer","page_kind_confidence":0.99,"classification_reason":"answer","ocr_confidence":0.8,"ocr_issues":[],"printed_questions":[],"questions":[{"boundary":"new_answer","boundary_confidence":1,"visible_label":"INDIA'S STANCE","title":"","answer_markdown":"page five model rewrite","status":"needs review"}]}`,
		`{"decisions":[{"page_number":2,"boundary":"new_answer","boundary_confidence":0.99,"visible_label":"1(a)","label_evidence":"1(a)","reason":"visible answer label"},{"page_number":3,"boundary":"continuation","boundary_confidence":0.97,"visible_label":"","label_evidence":"","reason":"continues the preceding argument"},{"page_number":4,"boundary":"new_answer","boundary_confidence":0.99,"visible_label":"1(b)","label_evidence":"1(b)","reason":"visible answer label and topic reset"},{"page_number":5,"boundary":"continuation","boundary_confidence":0.97,"visible_label":"","label_evidence":"","reason":"continues the CRISPR answer"}]}`,
	}}
	pages := []Page{
		{Number: 2, Text: "1(a) original attitude OCR"},
		{Number: 3, Text: "original attitude continuation OCR"},
		{Number: 4, Text: "1(b) original CRISPR OCR"},
		{Number: 5, Text: "original CRISPR continuation OCR"},
	}
	service := New(config.ToolConfig{}, &fakeRunner{}, provider)
	result, err := service.splitQuestions(
		context.Background(),
		pages,
		questionSplitOptions{QuestionModel: "local-model", Workers: 1},
		nil,
	)
	if err != nil {
		t.Fatalf("splitQuestions() error = %v", err)
	}
	if len(result.Questions) != 2 {
		t.Fatalf("questions = %#v, want two ledger-grouped answers", result.Questions)
	}
	if result.Questions[0].Label != "1(a)" || !slices.Equal(result.Questions[0].SourcePages, []int{2, 3}) {
		t.Fatalf("first question = %#v, want 1(a) on pages 2-3", result.Questions[0])
	}
	if result.Questions[1].Label != "1(b)" || !slices.Equal(result.Questions[1].SourcePages, []int{4, 5}) {
		t.Fatalf("second question = %#v, want 1(b) on pages 4-5", result.Questions[1])
	}
	if !strings.Contains(result.Questions[0].AnswerMarkdown, "original attitude continuation OCR") || strings.Contains(result.Questions[0].AnswerMarkdown, "model rewrite") {
		t.Fatalf("answer markdown = %q, want original OCR only", result.Questions[0].AnswerMarkdown)
	}
}

func TestAnswerBearingPagesExcludeCoverAndOCRFailures(t *testing.T) {
	t.Parallel()

	pages := answerBearingPages([]Page{
		{
			Number: 1,
			Kind:   "cover",
			Text:   "ForumIAS ACADEMY\nName Of Candidate\nINDEX TABLE\nINSTRUCTIONS\nMaximum Marks",
		},
		{
			Number: 2,
			Text:   "> OCR failed for this page: OCR response was empty",
		},
		{
			Number: 3,
			Kind:   "question_paper",
			Text:   "Instructions: All questions are compulsory. Duration 3 Hours. 20 Questions | 250 Marks. Section A. Q.1 What is attitude?",
		},
		{
			Number: 4,
			Kind:   "answer",
			Text:   "Q.2 answer body with useful content",
		},
	})
	if len(pages) != 1 || pages[0].Number != 4 {
		t.Fatalf("answerBearingPages() = %#v, want only page 4", pages)
	}
}

func TestQuestionSplitPagesRetainPrintedQuestionLedgerOnOCRReuse(t *testing.T) {
	t.Parallel()

	pages := pagesForQuestionSplit([]Page{
		{Number: 1, Kind: "cover", Text: "cover"},
		{Number: 2, Kind: "question_paper", Text: "printed questions"},
		{Number: 3, Kind: "answer", Text: "candidate answer"},
		{Number: 4, Kind: "blank", Text: ""},
		{Number: 5, Kind: "unknown", Text: "unclassified page"},
		{Number: 6, Text: "> OCR failed for this page: empty"},
	})
	if len(pages) != 3 || pages[0].Number != 2 || pages[1].Number != 3 || pages[2].Number != 5 {
		t.Fatalf("pagesForQuestionSplit() = %#v, want question paper, answer, and unknown pages", pages)
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
	fp := &fakeProvider{visionContent: "Q1 page text", chatResponses: []string{
		`{"page_kind":"answer","page_kind_confidence":0.98,"ocr_confidence":0.95,"questions":[{"boundary":"new_answer","boundary_confidence":0.98,"visible_label":"Q1","title":"Polity","answer_markdown":"answer block","status":"detected"}]}`,
		`{"decisions":[{"page_number":1,"boundary":"new_answer","boundary_confidence":0.98,"visible_label":"Q1","label_evidence":"Q1","reason":"visible label"}]}`,
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
	if len(fp.chatPrompts) != 4 {
		t.Fatalf("chat calls = %d, want page classification + boundary ledger + dimensions + report", len(fp.chatPrompts))
	}
	if !strings.Contains(fp.chatPrompts[0], "Classify OCR page") {
		t.Fatalf("first chat prompt = %q, want question split", fp.chatPrompts[0])
	}
	if !strings.Contains(fp.chatPrompts[1], "answer-boundary ledger") {
		t.Fatalf("second chat prompt = %q, want copy-level boundary ledger", fp.chatPrompts[1])
	}
	if !strings.Contains(fp.chatPrompts[2], "evidence-based diagnostic") {
		t.Fatalf("third chat prompt = %q, want dimensions", fp.chatPrompts[2])
	}
	for _, want := range []string{"Structured per-question analysis", `"introduction": "good"`} {
		if !strings.Contains(fp.chatPrompts[3], want) {
			t.Fatalf("report prompt missing %q:\n%s", want, fp.chatPrompts[3])
		}
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
	fp := &fakeProvider{chatResponses: []string{
		"",
		"",
		`{"decisions":[{"page_number":1,"boundary":"uncertain","boundary_confidence":0.2,"visible_label":"","label_evidence":"","reason":"page classification failed"}]}`,
		"final report",
	}}
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
	if question.Label != "Page 1 block" || question.Status != "needs review" || question.AnswerMarkdown != "page text" {
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
	ocrProvider := &fakeProvider{id: "ocr", visionContent: "Q1 ocr page text"}
	questionProvider := &fakeProvider{id: "question", chatResponses: []string{
		`{"page_kind":"answer","page_kind_confidence":0.98,"ocr_confidence":0.95,"questions":[{"boundary":"new_answer","boundary_confidence":0.98,"visible_label":"Q1","answer_markdown":"question answer","status":"detected"}]}`,
		`{"decisions":[{"page_number":1,"boundary":"new_answer","boundary_confidence":0.98,"visible_label":"Q1","label_evidence":"Q1","reason":"visible label"}]}`,
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
		BoundaryModel: "boundary-model",
		ReportModel:   "report-model",
		QuestionSplit: true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Report != "report text" || len(res.Questions) != 1 || res.Questions[0].AnswerMarkdown != "Q1 ocr page text" {
		t.Fatalf("Response = %#v, want split question and report", res)
	}
	if ocrProvider.visionPrompt == "" {
		t.Fatal("ocr provider was not used")
	}
	if !strings.Contains(questionProvider.chatPrompts[0], "Classify OCR page") {
		t.Fatalf("question provider prompt = %q, want question split prompt", questionProvider.chatPrompts[0])
	}
	if !strings.Contains(questionProvider.chatPrompts[1], "answer-boundary ledger") {
		t.Fatalf("question provider prompt = %q, want boundary-ledger prompt", questionProvider.chatPrompts[1])
	}
	if !strings.Contains(questionProvider.chatPrompts[2], "evidence-based diagnostic") {
		t.Fatalf("question provider prompt = %q, want dimensions prompt", questionProvider.chatPrompts[2])
	}
	if !slices.Equal(questionProvider.chatModels, []string{"split-model", "boundary-model", "split-model"}) {
		t.Fatalf("question provider models = %#v, want classification, boundary, and analysis routing", questionProvider.chatModels)
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

func TestParsePageQuestionSplitClassifiesQuestionPaperWithoutAnswerBlocks(t *testing.T) {
	t.Parallel()

	result, err := parsePageQuestionSplit(`{
		"page_kind":"question_paper",
		"page_kind_confidence":0.96,
		"classification_reason":"The page lists printed questions and contains no candidate writing.",
		"printed_questions":[
			{"label":"Q.1 a)","prompt":"What is attitude? Explain with examples. (10 marks, 150 words)"},
			{"label":"Q1(b)","prompt":"Identify ethical issues in genetic manipulation."}
		],
		"questions":[]
	}`, 1)
	if err != nil {
		t.Fatalf("parsePageQuestionSplit() error = %v", err)
	}
	if result.Classification.Kind != "question_paper" || result.Classification.Confidence != 0.96 {
		t.Fatalf("classification = %#v, want semantic question-paper result", result.Classification)
	}
	if len(result.PrintedQuestions) != 2 || len(result.Questions) != 0 {
		t.Fatalf("result = %#v, want prompt ledger without answer blocks", result)
	}
}

func TestSplitPageQuestionsRetriesMalformedSemanticResponse(t *testing.T) {
	t.Parallel()

	ocrConfidence := 0.62
	provider := &fakeProvider{chatResponses: []string{
		"not valid JSON",
		`{
			"page_kind":"question_paper",
			"page_kind_confidence":0.98,
			"classification_reason":"Printed questions without candidate answers.",
			"ocr_confidence":0.62,
			"ocr_issues":["One directive word is uncertain"],
			"printed_questions":[{"label":"Q1(a)","prompt":"What is attitude? Explain with examples."}],
			"questions":[]
		}`,
	}}
	service := New(config.ToolConfig{}, &fakeRunner{}, provider)
	result, err := service.splitPageQuestions(context.Background(), "local-model", Page{Number: 1, Text: "printed OCR"})
	if err != nil {
		t.Fatalf("splitPageQuestions() error = %v", err)
	}
	if len(provider.chatPrompts) != 2 || !strings.Contains(provider.chatPrompts[1], "Retry the semantic classification") {
		t.Fatalf("prompts = %#v, want one focused model retry", provider.chatPrompts)
	}
	for index, schema := range provider.chatSchemas {
		if schema == nil || schema.Name != "topper_copy_page" || !schema.Strict {
			t.Fatalf("schema %d = %#v, want strict page output contract", index, schema)
		}
	}
	if result.Classification.Kind != "question_paper" || len(result.PrintedQuestions) != 1 {
		t.Fatalf("result = %#v, want recovered question-paper ledger", result)
	}
	if result.Classification.OCRConfidence == nil || *result.Classification.OCRConfidence != ocrConfidence || len(result.Classification.OCRIssues) != 1 {
		t.Fatalf("classification = %#v, want semantic OCR reliability", result.Classification)
	}
}

func TestParseQuestionDimensionsNormalizesRichAnalysis(t *testing.T) {
	t.Parallel()

	content := "```json\n" + `{
		"demand_alignment":"  Directly answers the evaluate directive.  ",
		"fact":["Article 21","NCRB data"],
		"strengths":[
			{"point":"Clear structure","evidence":"Three visible headings","why_it_matters":"Easy to scan"},
			{"point":"clear structure","evidence":"duplicate"}
		],
		"gaps":[{"point":"Thin counterpoint","evidence":"Only one opposing line"}],
		"missing_dimensions":["Stakeholder view","stakeholder view"],
		"improvements":[{"priority":"urgent","change":"Add a proportionate counterpoint"}],
		"reusable_techniques":["Heading-led body"],
		"scorecard":{
			"demand_fulfilment":8,
			"structure":4,
			"overall_percent":120,
			"estimated_band":"strong",
			"confidence":"medium",
			"rationale":"Visible demand and structure support the result."
		}
	}` + "\n```"

	dimensions, err := parseQuestionDimensions(content)
	if err != nil {
		t.Fatalf("parseQuestionDimensions() error = %v", err)
	}
	if dimensions.DemandAlignment != "Directly answers the evaluate directive." {
		t.Fatalf("demand alignment = %q, want trimmed analysis", dimensions.DemandAlignment)
	}
	if dimensions.Fact != "Article 21; NCRB data" {
		t.Fatalf("fact = %q, want array normalized to readable text", dimensions.Fact)
	}
	if len(dimensions.Strengths) != 1 || dimensions.Strengths[0].Evidence != "Three visible headings" {
		t.Fatalf("strengths = %#v, want case-insensitive de-duplication", dimensions.Strengths)
	}
	if len(dimensions.MissingDimensions) != 1 {
		t.Fatalf("missing dimensions = %#v, want de-duplicated list", dimensions.MissingDimensions)
	}
	if len(dimensions.Improvements) != 1 || dimensions.Improvements[0].Priority != "medium" {
		t.Fatalf("improvements = %#v, want invalid priority normalized", dimensions.Improvements)
	}
	if dimensions.Scorecard == nil || dimensions.Scorecard.DemandFulfilment != 5 || dimensions.Scorecard.OverallPercent != 100 {
		t.Fatalf("scorecard = %#v, want scores clamped to rubric bounds", dimensions.Scorecard)
	}
}

func TestQuestionReferenceNormalizesPrintedAndAnswerLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		label string
		want  string
	}{
		{name: "printed dotted subpart", label: "Q.1 a)", want: "1a"},
		{name: "answer parenthesized subpart", label: "1(b)", want: "1b"},
		{name: "question word", label: "Question 12(c)", want: "12c"},
		{name: "plain question", label: "Q7", want: "7"},
		{name: "continuation is not a reference", label: "Page 3 continuation", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := questionReference(tt.label); got != tt.want {
				t.Fatalf("questionReference(%q) = %q, want %q", tt.label, got, tt.want)
			}
		})
	}
}

func TestAttachPrintedQuestionPromptsMapsSubpartVariants(t *testing.T) {
	t.Parallel()

	questions := attachPrintedQuestionPrompts(
		[]Question{
			{Label: "Q1(a)"},
			{Label: "1(b)", Title: "ETHICS ASSOCIATED"},
		},
		[]PrintedQuestion{
			{Label: "Q1(a)", Prompt: "What is attitude? Explain with examples. (10 marks, 150 words)"},
			{Label: "Q1(b)", Prompt: "Identify ethical issues in genetic manipulation."},
		},
	)
	if questions[0].Title != "What is attitude? Explain with examples. (10 marks, 150 words)" {
		t.Fatalf("Q1(a) title = %q, want exact printed prompt", questions[0].Title)
	}
	if questions[1].Title != "Identify ethical issues in genetic manipulation." {
		t.Fatalf("1(b) title = %q, want printed prompt to replace answer heading", questions[1].Title)
	}
}

func TestAnalysisQualityReportsCoverageWithoutClaimingAccuracy(t *testing.T) {
	t.Parallel()

	highOCRConfidence := 0.9
	lowOCRConfidence := 0.5
	quality := analysisQuality(
		[]Page{
			{Number: 1, Kind: "answer", KindConfidence: 0.9, OCRConfidence: &highOCRConfidence, Text: "clear answer text"},
			{Number: 2, Kind: "question_paper", KindConfidence: 0.7, OCRConfidence: &lowOCRConfidence, Text: "printed prompt [unclear]"},
		},
		[]Question{
			{
				Title: "Exact printed prompt",
				Dimensions: &QuestionDimensions{
					Strengths: []AnalysisPoint{{Point: "Structured", Evidence: "Uses three headings"}},
					Gaps:      []AnalysisPoint{{Point: "Thin counterpoint", Evidence: "One opposing line"}},
				},
			},
			{Dimensions: &QuestionDimensions{Strengths: []AnalysisPoint{{Point: "Readable"}}}},
		},
	)

	if quality.ClassificationCoveragePercent != 100 || quality.AverageClassificationConfidence != 0.8 {
		t.Fatalf("classification quality = %#v, want full coverage at 0.8 average confidence", quality)
	}
	if quality.OCRAssessmentCoveragePercent != 100 || quality.AverageOCRConfidence != 0.7 {
		t.Fatalf("OCR quality = %#v, want independently assessed semantic confidence", quality)
	}
	if quality.MinimumOCRConfidence != 0.5 || !slices.Equal(quality.OCRReviewPages, []int{2}) {
		t.Fatalf("OCR review details = %#v, want the weakest page identified", quality)
	}
	if quality.PromptMatchPercent != 50 || quality.AnalysisCoveragePercent != 100 || quality.EvidenceCoveragePercent != 50 {
		t.Fatalf("analysis quality = %#v, want independently reported coverage metrics", quality)
	}
	if !quality.RequiresReview || len(quality.Warnings) == 0 {
		t.Fatalf("quality = %#v, want review warning for incomplete prompt/evidence coverage", quality)
	}
}

func TestAnalysisQualityDoesNotLetAverageHideUnreliableOCRPages(t *testing.T) {
	t.Parallel()

	highConfidence := 0.98
	lowConfidence := 0.1
	tests := []struct {
		name               string
		pages              []Page
		want               []int
		averageAppearsSafe bool
	}{
		{
			name: "one low confidence page",
			pages: []Page{
				{Number: 1, Kind: "answer", KindConfidence: 0.95, OCRConfidence: &highConfidence, Text: "clear"},
				{Number: 2, Kind: "answer", KindConfidence: 0.95, OCRConfidence: &lowConfidence, Text: "damaged"},
				{Number: 3, Kind: "answer", KindConfidence: 0.95, OCRConfidence: &highConfidence, Text: "clear"},
				{Number: 4, Kind: "answer", KindConfidence: 0.95, OCRConfidence: &highConfidence, Text: "clear"},
			},
			want:               []int{2},
			averageAppearsSafe: true,
		},
		{
			name: "reported issue despite high confidence",
			pages: []Page{
				{
					Number:         4,
					Kind:           "answer",
					KindConfidence: 0.95,
					OCRConfidence:  &highConfidence,
					OCRIssues:      []string{"A portion of the answer is unreadable"},
					Text:           "partially reliable",
				},
			},
			want: []int{4},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			quality := analysisQuality(tt.pages, []Question{{
				Title: "Exact prompt",
				Dimensions: &QuestionDimensions{
					Strengths: []AnalysisPoint{{Point: "Relevant", Evidence: "Direct answer"}},
				},
			}})
			if !quality.RequiresReview || !slices.Equal(quality.OCRReviewPages, tt.want) {
				t.Fatalf("quality = %#v, want OCR review pages %v", quality, tt.want)
			}
			if tt.averageAppearsSafe && quality.AverageOCRConfidence < 0.7 {
				t.Fatalf("average OCR confidence = %v, test must exercise a deceptively safe average", quality.AverageOCRConfidence)
			}
			if !strings.Contains(strings.Join(quality.Warnings, " "), "model-reported OCR issues") {
				t.Fatalf("warnings = %#v, want explicit OCR reliability warning", quality.Warnings)
			}
		})
	}
}

func TestAnalysisQualityPenalizesLowClassificationConfidence(t *testing.T) {
	t.Parallel()

	quality := analysisQuality(
		[]Page{{Number: 1, Kind: "answer", KindConfidence: 0.2, Text: "answer"}},
		[]Question{{
			Title: "Prompt",
			Dimensions: &QuestionDimensions{
				Strengths: []AnalysisPoint{{Point: "Relevant", Evidence: "Direct opening"}},
			},
		}},
	)

	if quality.OverallCoveragePercent >= 90 {
		t.Fatalf("overall coverage = %d, want low model confidence reflected in aggregate", quality.OverallCoveragePercent)
	}
	if !quality.RequiresReview || !strings.Contains(strings.Join(quality.Warnings, " "), "low model confidence") {
		t.Fatalf("quality = %#v, want explicit low-confidence review warning", quality)
	}
}

func TestAnalysisQualityExplainsUncertainAnswerBoundaries(t *testing.T) {
	t.Parallel()

	quality := analysisQuality(
		[]Page{{Number: 1, Kind: "answer", KindConfidence: 0.95, Text: "answer text"}},
		[]Question{{
			Label:              "Page 1 block",
			AnswerMarkdown:     "answer text",
			SourcePages:        []int{1},
			Status:             "needs review",
			Boundary:           questionBoundaryUncertain,
			BoundaryConfidence: 0.4,
		}},
	)
	if !quality.RequiresReview {
		t.Fatal("RequiresReview = false, want uncertain boundary review")
	}
	warnings := strings.Join(quality.Warnings, " ")
	if !strings.Contains(warnings, "answer boundaries") {
		t.Fatalf("warnings = %q, want explicit answer-boundary warning", warnings)
	}
}

func TestExtractDimensionsPreservesExistingAnalysisOnInvalidResponse(t *testing.T) {
	t.Parallel()

	existing := &QuestionDimensions{Introduction: "verified introduction"}
	service := New(
		config.ToolConfig{PDFToPPM: "pdftoppm"},
		&fakeRunner{},
		&fakeProvider{chatResponses: []string{"not JSON"}},
	)
	questions := service.extractDimensions(
		context.Background(),
		"model",
		[]Question{{
			ID:             "q1",
			Label:          "Q.1",
			AnswerMarkdown: "answer text",
			Status:         "detected",
			Dimensions:     existing,
		}},
		1,
		nil,
	)
	if len(questions) != 1 || questions[0].Dimensions == nil || questions[0].Dimensions.Introduction != "verified introduction" {
		t.Fatalf("questions = %#v, want existing analysis preserved", questions)
	}
}

func TestParseOneShotPDFManifest(t *testing.T) {
	t.Parallel()

	content := "```json\n{\"metadata\":{\"topper_name\":\"A Topper\",\"paper\":\"GS1\"},\"detected_questions\":[\"Q.1\"],\"pages\":[{\"number\":1,\"name\":\"page-1\",\"text\":\"ocr text\",\"unclear_count\":1}],\"questions\":[{\"label\":\"Q.1\",\"title\":\"History\",\"source_pages\":[1],\"answer_markdown\":\"test answer\",\"dimensions\":{\"fact\":\"good examples\",\"strengths\":[{\"point\":\"specific example\",\"evidence\":\"visible case\"}],\"scorecard\":{\"evidence\":4,\"overall_percent\":76,\"estimated_band\":\"strong\",\"confidence\":\"high\"}},\"metadata\":{\"subject\":\"History\",\"topic\":\"Ancient India\",\"marks\":10}}],\"report\":\"test report\"}\n```"
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
	if len(questions[0].Dimensions.Strengths) != 1 || questions[0].Dimensions.Scorecard == nil || questions[0].Dimensions.Scorecard.OverallPercent != 76 {
		t.Fatalf("dimensions = %#v, want rich analysis fields", questions[0].Dimensions)
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
