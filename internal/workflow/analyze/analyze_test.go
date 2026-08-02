package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct {
	args      []string
	pageCount int
}

func (r *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	r.args = append([]string(nil), args...)
	switch filepath.Base(command) {
	case "pdfinfo":
		pageCount := r.pageCount
		if pageCount <= 0 {
			pageCount = 1
		}
		return []byte(fmt.Sprintf("Pages: %d\n", pageCount)), nil
	case "qpdf":
		if len(args) < 2 {
			return nil, errors.New("qpdf test invocation has no output path")
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			return nil, err
		}
		return []byte("ok"), os.WriteFile(args[len(args)-1], data, 0o600)
	}
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
	documentPrompts   []string
	documentModels    []string
	documentRequest   provider.DocumentRequest
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
	p.documentPrompts = append(p.documentPrompts, req.Prompt)
	p.documentModels = append(p.documentModels, req.Model)
	p.documentRequest = req
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
	if provider.documentRequest.ResponseMIMEType != "application/json" {
		t.Fatalf("response MIME type = %q, want application/json", provider.documentRequest.ResponseMIMEType)
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

func TestPlanDirectPDFChunksUsesEightPagesWithTwoPageOverlap(t *testing.T) {
	t.Parallel()

	chunks, err := planDirectPDFChunks(20)
	if err != nil {
		t.Fatalf("planDirectPDFChunks() error = %v", err)
	}
	want := []directPDFChunk{
		{Index: 0, FirstPage: 1, LastPage: 8},
		{Index: 1, FirstPage: 7, LastPage: 14},
		{Index: 2, FirstPage: 13, LastPage: 20},
	}
	if !slices.Equal(chunks, want) {
		t.Fatalf("chunks = %#v, want %#v", chunks, want)
	}
}

func TestDirectPDFChunkFallsBackToSmallerOverlappingRanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pdf := filepath.Join(dir, "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := `{"detected_questions":["Q.1"],"pages":[`
	firstHalf := `{
		"metadata":{"topper_name":"Sample Topper"},
		"detected_questions":["Q.1"],
		"pages":[
			{"number":1,"kind":"cover"},{"number":2,"kind":"answer"},{"number":3,"kind":"answer"},
			{"number":4,"kind":"answer"},{"number":5,"kind":"answer"}
		],
		"questions":[{"id":"q1","label":"Q.1","source_pages":[2],"answer_markdown":"first answer"}],
		"report":"first half"
	}`
	secondHalf := `{
		"metadata":{"topper_name":"Sample Topper"},
		"detected_questions":["Q.2"],
		"pages":[
			{"number":1,"kind":"answer"},{"number":2,"kind":"answer"},
			{"number":3,"kind":"answer"},{"number":4,"kind":"answer"}
		],
		"questions":[{"id":"q2","label":"Q.2","source_pages":[2],"answer_markdown":"second answer"}],
		"report":"second half"
	}`
	processor := &fakeProvider{
		documentResponses: []string{invalid, invalid, firstHalf, secondHalf},
	}
	service := New(
		config.ToolConfig{QPDF: "qpdf"},
		&fakeRunner{},
		processor,
	)
	chunk := directPDFChunk{Index: 0, FirstPage: 1, LastPage: 8}
	result, err := service.extractDirectPDFChunkWithFallback(
		context.Background(),
		Request{Path: pdf, OCRModel: "gemini-flash-lite-latest"},
		"answers.pdf",
		10,
		chunk,
		dir,
		directPDFChunkPrompts("answers.pdf", 10, chunk),
		processor,
	)
	if err != nil {
		t.Fatalf("extractDirectPDFChunkWithFallback() error = %v", err)
	}
	if processor.documentCalls != 4 || result.Response.APICalls != 4 {
		t.Fatalf("calls: provider=%d response=%d, want two rejected parent calls plus two smaller ranges", processor.documentCalls, result.Response.APICalls)
	}
	if len(result.Response.Pages) != 8 || len(result.Response.Questions) != 2 {
		t.Fatalf("response has %d pages and %d questions, want 8 and 2", len(result.Response.Pages), len(result.Response.Questions))
	}
	if !slices.Equal(result.Response.Questions[0].SourcePages, []int{2}) ||
		!slices.Equal(result.Response.Questions[1].SourcePages, []int{6}) {
		t.Fatalf("question source pages = %#v, want globally mapped pages 2 and 6", result.Response.Questions)
	}
}

func TestDirectPDFChunkFallbackRecursesUntilValid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pdf := filepath.Join(dir, "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := `{"detected_questions":["Q.1"],"pages":[`
	firstHalf := `{
		"metadata":{"topper_name":"Sample Topper"},
		"detected_questions":["Q.1"],
		"pages":[
			{"number":1,"kind":"cover"},{"number":2,"kind":"answer"},{"number":3,"kind":"answer"},
			{"number":4,"kind":"answer"},{"number":5,"kind":"answer"}
		],
		"questions":[{"id":"q1","label":"Q.1","source_pages":[2],"answer_markdown":"first answer"}],
		"report":"first half"
	}`
	nestedFirst := `{
		"metadata":{"topper_name":"Sample Topper"},
		"detected_questions":["Q.2"],
		"pages":[{"number":1,"kind":"answer"},{"number":2,"kind":"answer"},{"number":3,"kind":"answer"}],
		"questions":[{"id":"q2","label":"Q.2","source_pages":[2],"answer_markdown":"nested first"}],
		"report":"nested first"
	}`
	nestedSecond := `{
		"metadata":{"topper_name":"Sample Topper"},
		"detected_questions":["Q.3"],
		"pages":[{"number":1,"kind":"answer"},{"number":2,"kind":"answer"}],
		"questions":[{"id":"q3","label":"Q.3","source_pages":[2],"answer_markdown":"nested second"}],
		"report":"nested second"
	}`
	processor := &fakeProvider{
		documentResponses: []string{
			invalid,
			invalid,
			firstHalf,
			invalid,
			invalid,
			nestedFirst,
			nestedSecond,
		},
	}
	service := New(
		config.ToolConfig{QPDF: "qpdf"},
		&fakeRunner{},
		processor,
	)
	chunk := directPDFChunk{Index: 0, FirstPage: 1, LastPage: 8}
	result, err := service.extractDirectPDFChunkWithFallback(
		context.Background(),
		Request{Path: pdf, OCRModel: "gemini-flash-lite-latest"},
		"answers.pdf",
		10,
		chunk,
		dir,
		directPDFChunkPrompts("answers.pdf", 10, chunk),
		processor,
	)
	if err != nil {
		t.Fatalf("extractDirectPDFChunkWithFallback() error = %v", err)
	}
	if processor.documentCalls != 7 || result.Response.APICalls != 7 {
		t.Fatalf("calls: provider=%d response=%d, want rejected parent and child attempts plus three valid ranges", processor.documentCalls, result.Response.APICalls)
	}
	if len(result.Response.Pages) != 8 || len(result.Response.Questions) != 3 {
		t.Fatalf("response has %d pages and %d questions, want 8 and 3", len(result.Response.Pages), len(result.Response.Questions))
	}
	if !slices.Equal(result.Response.Questions[0].SourcePages, []int{2}) ||
		!slices.Equal(result.Response.Questions[1].SourcePages, []int{6}) ||
		!slices.Equal(result.Response.Questions[2].SourcePages, []int{8}) {
		t.Fatalf("question source pages = %#v, want recursively mapped pages 2, 6, and 8", result.Response.Questions)
	}
}

func TestSplitDirectPDFChunkForFallbackReachesSinglePageFloor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		chunk directPDFChunk
		want  []directPDFChunk
		ok    bool
	}{
		{
			name:  "single page cannot split",
			chunk: directPDFChunk{Index: 2, FirstPage: 7, LastPage: 7},
			ok:    false,
		},
		{
			name:  "two pages split into single pages",
			chunk: directPDFChunk{Index: 2, FirstPage: 7, LastPage: 8},
			want: []directPDFChunk{
				{Index: 2, FirstPage: 7, LastPage: 7},
				{Index: 2, FirstPage: 8, LastPage: 8},
			},
			ok: true,
		},
		{
			name:  "three pages retain one-page overlap",
			chunk: directPDFChunk{Index: 2, FirstPage: 7, LastPage: 9},
			want: []directPDFChunk{
				{Index: 2, FirstPage: 7, LastPage: 8},
				{Index: 2, FirstPage: 8, LastPage: 9},
			},
			ok: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := splitDirectPDFChunkForFallback(tt.chunk)
			if ok != tt.ok || !slices.Equal(got, tt.want) {
				t.Fatalf("splitDirectPDFChunkForFallback(%#v) = (%#v, %t), want (%#v, %t)", tt.chunk, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestDirectPDFPromptsKeepProcessingBoundariesOutOfVisibleEvidence(t *testing.T) {
	t.Parallel()

	chunkPrompt := directPDFChunkPrompt("copy.pdf", 56, directPDFChunk{Index: 1, FirstPage: 7, LastPage: 14}, false)
	for _, instruction := range []string{
		"internal processing detail, not evidence",
		"Never mention chunks, chunk edges, extraction windows, or technical processing as a content cause",
		"Describe only visible page evidence",
	} {
		if !strings.Contains(chunkPrompt, instruction) {
			t.Fatalf("chunk prompt missing semantic causality instruction %q", instruction)
		}
	}

	reconciliationPrompt := directPDFReconciliationPrompt("copy.pdf", "[]", "{}", 56, "", nil)
	for _, instruction := range []string{
		"internal processing details—not visible-copy evidence",
		"Never mention them in group reasons, warnings, answer analyses, or report text",
		"Describe only what the attached PDF visibly shows",
		"comfortably below 40,000 output tokens",
		"180 words for each answer analysis",
		"Do not repeat or quote the full answer inside analysis/report prose",
	} {
		if !strings.Contains(reconciliationPrompt, instruction) {
			t.Fatalf("reconciliation prompt missing semantic causality instruction %q", instruction)
		}
	}
}

func TestDirectPDFVisibleAnalysisRejectsInternalProcessingExplanations(t *testing.T) {
	t.Parallel()

	questions := []Question{{
		ID:    "q20",
		Label: "Q.20",
		Dimensions: &QuestionDimensions{
			Custom: "Answer incomplete due to chunk boundary.",
		},
	}}
	if err := validateDirectPDFVisibleAnalysis(questions, "visible report", nil); err == nil {
		t.Fatal("analysis attributed to an internal chunk boundary was accepted")
	}
	questions[0].Dimensions.Custom = "Writing visibly stops mid-sentence on page 53; pages 54-56 are blank."
	if err := validateDirectPDFVisibleAnalysis(questions, "visible report", nil); err != nil {
		t.Fatalf("visible page evidence was rejected: %v", err)
	}
	if err := validateDirectPDFVisibleAnalysis(questions, "technical processing caused truncation", nil); err == nil {
		t.Fatal("copy-wide report attributed content quality to technical processing")
	}
}

func TestDirectPDFInternalProcessingAnalysisIsRefreshedFromEvidence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pdf := filepath.Join(dir, "answers.pdf")
	if err := os.WriteFile(pdf, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	processor := &fakeProvider{documentResponses: []string{
		`{"custom":"Incomplete due to chunk boundary."}`,
		`{"custom":"Writing visibly stops mid-sentence on global page 53."}`,
	}}
	service := New(config.ToolConfig{QPDF: "qpdf"}, &fakeRunner{}, processor)
	questions, usage, calls, err := service.refreshDirectPDFInternalProcessingAnalysis(
		context.Background(),
		Request{Path: pdf},
		"gemini-flash-lite-latest",
		dir,
		[]Question{{
			ID:             "q20",
			Label:          "Q.20",
			Title:          "Afghanistan connectivity",
			AnswerMarkdown: "2. Promote",
			SourcePages:    []int{53},
			Dimensions:     &QuestionDimensions{Custom: "Fragmentary due to extraction boundary."},
		}},
		processor,
	)
	if err != nil {
		t.Fatalf("refreshDirectPDFInternalProcessingAnalysis() error = %v", err)
	}
	if usage != nil || calls != 2 || processor.documentCalls != 2 {
		t.Fatalf("usage=%#v calls=%d provider calls=%d, want two content attempts", usage, calls, processor.documentCalls)
	}
	if got := questions[0].Dimensions.Custom; got != "Writing visibly stops mid-sentence on global page 53." {
		t.Fatalf("refreshed custom analysis = %q", got)
	}
	if len(processor.documentPrompts) != 2 || !strings.Contains(processor.documentPrompts[1], "previous analysis was rejected") ||
		!strings.Contains(processor.documentPrompts[1], "chunk boundary") {
		t.Fatalf("strict refresh prompt missing validator feedback: %#v", processor.documentPrompts)
	}
	if processor.documentRequest.ResponseSchema == nil || processor.documentRequest.ResponseSchema.Name != "topper_question_analysis" {
		t.Fatalf("refresh response schema = %#v", processor.documentRequest.ResponseSchema)
	}
}

func TestDirectPDFReconciliationRejectsAnsweredUnansweredPageOverlap(t *testing.T) {
	t.Parallel()

	err := validateDirectPDFAnsweredUnansweredOverlap([]directPDFReconciliationGroup{
		{ID: "q8-prompt", Status: directPDFQuestionUnanswered, SourcePages: []int{32}},
		{ID: "q8-answer", Status: directPDFQuestionAnswered, SourcePages: []int{32, 33, 34, 35, 36}},
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be both answered and unanswered") {
		t.Fatalf("answered/unanswered overlap error = %v", err)
	}
	if err := validateDirectPDFAnsweredUnansweredOverlap([]directPDFReconciliationGroup{
		{ID: "q8", Status: directPDFQuestionAnswered, SourcePages: []int{32, 33, 34, 35, 36}},
		{ID: "q9", Status: directPDFQuestionAnswered, SourcePages: []int{36, 37, 38}},
	}); err != nil {
		t.Fatalf("adjacent answered groups sharing a physical page were rejected: %v", err)
	}
}

func TestDirectPDFReconciliationQuotaDelayUsesWholeMinuteWindow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 2, 12, 0, 0, 0, time.UTC)
	if delay := directPDFReconciliationQuotaDelay(time.Time{}, now); delay != 0 {
		t.Fatalf("first reconciliation delay = %v, want zero", delay)
	}
	if delay := directPDFReconciliationQuotaDelay(now.Add(-10*time.Second), now); delay != 51*time.Second {
		t.Fatalf("reconciliation inside quota window delay = %v, want 51s", delay)
	}
	if delay := directPDFReconciliationQuotaDelay(now.Add(-directPDFReconciliationQuotaWindow), now); delay != 0 {
		t.Fatalf("reconciliation after quota window delay = %v, want zero", delay)
	}
}

func TestDirectPDFChunkCheckpointTracksActualPromptFingerprint(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	chunk := directPDFChunk{Index: 0, FirstPage: 1, LastPage: 8}
	prompts := directPDFChunkPrompts("copy.pdf", 10, chunk)
	promptSHA256 := directPDFPromptsSHA256(prompts)
	if promptSHA256 != directPDFPromptsSHA256(directPDFChunkPrompts("copy.pdf", 10, chunk)) {
		t.Fatal("unchanged direct PDF prompts produced different fingerprints")
	}
	changedPrompts := append([]string(nil), prompts...)
	changedPrompts[1] += "\nSchema changed."
	changedPromptSHA256 := directPDFPromptsSHA256(changedPrompts)
	if changedPromptSHA256 == promptSHA256 {
		t.Fatal("changed direct PDF prompt/schema produced the same fingerprint")
	}

	checkpoint := directPDFChunkCheckpoint{
		Version:      directPDFCheckpointVersion,
		SourceSHA256: "source-sha",
		Model:        "model",
		PromptSHA256: promptSHA256,
		Result: directPDFChunkResult{
			Chunk:    chunk,
			Response: Response{Report: "original"},
		},
	}
	if err := saveDirectPDFChunkCheckpoint(dir, checkpoint); err != nil {
		t.Fatalf("saveDirectPDFChunkCheckpoint() error = %v", err)
	}
	loaded, found, err := loadDirectPDFChunkCheckpoint(dir, chunk, "source-sha", "model", promptSHA256)
	if err != nil || !found || loaded.Response.Report != "original" {
		t.Fatalf("load unchanged checkpoint = (%#v, %v, %v), want original cache hit", loaded, found, err)
	}

	checkpoint.Result.Response.Report = "must not overwrite a reusable checkpoint"
	if err := saveDirectPDFChunkCheckpoint(dir, checkpoint); err != nil {
		t.Fatalf("save reusable checkpoint error = %v", err)
	}
	loaded, found, err = loadDirectPDFChunkCheckpoint(dir, chunk, "source-sha", "model", promptSHA256)
	if err != nil || !found || loaded.Response.Report != "original" {
		t.Fatalf("reloaded unchanged checkpoint = (%#v, %v, %v), want original cache entry", loaded, found, err)
	}

	if _, found, err = loadDirectPDFChunkCheckpoint(dir, chunk, "source-sha", "model", changedPromptSHA256); err != nil || found {
		t.Fatalf("load changed-prompt checkpoint = (found=%v, err=%v), want cache miss", found, err)
	}
	checkpoint.PromptSHA256 = changedPromptSHA256
	checkpoint.Result.Response.Report = "refreshed"
	if err := saveDirectPDFChunkCheckpoint(dir, checkpoint); err != nil {
		t.Fatalf("replace stale checkpoint error = %v", err)
	}
	loaded, found, err = loadDirectPDFChunkCheckpoint(dir, chunk, "source-sha", "model", changedPromptSHA256)
	if err != nil || !found || loaded.Response.Report != "refreshed" {
		t.Fatalf("load refreshed checkpoint = (%#v, %v, %v), want replaced cache entry", loaded, found, err)
	}

	checkpoint.PromptSHA256 = ""
	legacyJSON, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(directPDFChunkCheckpointPath(dir, chunk), legacyJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err = loadDirectPDFChunkCheckpoint(dir, chunk, "source-sha", "model", changedPromptSHA256); err != nil || found {
		t.Fatalf("load legacy checkpoint = (found=%v, err=%v), want cache miss", found, err)
	}
}

func TestRunAnalyzeChunkedDirectPDFReconcilesOverlapAndResumesChunks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pdf := filepath.Join(dir, "answers.pdf")
	if err := os.WriteFile(pdf, []byte("ten-page-pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	chunkOne := `{
		"metadata":{"topper_name":"Sample Topper","paper":"GS2","notes":"chunk one only"},
		"detected_questions":["Q.1","Q.2","Q.3"],
		"pages":[
			{"number":1,"kind":"cover"},{"number":2,"kind":"answer"},{"number":3,"kind":"answer"},{"number":4,"kind":"answer"},
			{"number":5,"kind":"evaluation"},{"number":6,"kind":"evaluation"},{"number":7,"kind":"answer"},{"number":8,"kind":"answer"}
		],
		"questions":[
			{"id":"q1","label":"Q.1","source_pages":[2,3,4],"answer_markdown":"complete first answer"},
			{"id":"q2-part","label":"Q.2","source_pages":[7,8],"answer_markdown":"partial second answer"},
			{"id":"q3-blank","label":"Q.3","title":"Third question","source_pages":[5],"answer_markdown":"Printed prompt; the visible answer area is blank."}
		],
		"report":"chunk one report"
	}`
	chunkTwo := `{
		"metadata":{"topper_name":"Sample Topper","paper":"GS2","notes":"chunk two only"},
		"detected_questions":["Q.2"],
		"pages":[
			{"number":1,"kind":"answer"},{"number":2,"kind":"answer"},{"number":3,"kind":"answer"},{"number":4,"kind":"answer"}
		],
		"questions":[
			{"id":"q2","label":"Q.2","source_pages":[1,2,3,4],"answer_markdown":"complete second answer","dimensions":{"introduction":"direct"}}
		],
		"report":"chunk two report"
	}`
	reconciliation := `{
		"groups":[
			{"id":"q1","status":"answered","candidate_ids":["chunk-001-question-001"],"canonical_candidate_id":"chunk-001-question-001","label":"Q.1","title":"First question","source_pages":[2,3,4],"merged_answer_markdown":"","confidence":0.99,"reason":"one complete internal answer"},
			{"id":"q2","status":"answered","candidate_ids":["chunk-001-question-003","chunk-002-question-001"],"canonical_candidate_id":"chunk-002-question-001","label":"Q.2","title":"Second question","source_pages":[7,8,9,10],"merged_answer_markdown":"","confidence":0.98,"reason":"overlap duplicate; second candidate covers the full answer"},
			{"id":"q3","status":"unanswered","candidate_ids":["chunk-001-question-002"],"canonical_candidate_id":"","label":"Q.3","title":"Third question","source_pages":[5],"merged_answer_markdown":"","confidence":0.97,"reason":"printed prompt with visibly blank answer area"}
		],
		"inventory":{"visible_question_slots":3,"answered":2,"unanswered":1},
		"warnings":[],
		"report":{
			"copy_profile":"copy profile",
			"scorecard_synthesis":"scorecard synthesis",
			"answer_analyses":[{"group_id":"q1","analysis":"first analysis"},{"group_id":"q2","analysis":"second analysis"}],
			"repeated_winning_patterns":"winning patterns",
			"what_not_to_copy_blindly":"copy risks",
			"gap_map":"gap map",
			"reusable_answer_writing_playbook":"playbook",
			"deliberate_practice_plan":"practice plan"
		}
	}`
	var invalidReconciliation directPDFReconciliation
	if err := json.Unmarshal([]byte(reconciliation), &invalidReconciliation); err != nil {
		t.Fatal(err)
	}
	invalidReconciliation.Inventory.Answered = 1
	invalidReconciliation.Inventory.Unanswered = 2
	invalidReconciliationJSON, err := json.Marshal(invalidReconciliation)
	if err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{
		id:                "gemini",
		documentResponses: []string{chunkOne, chunkTwo, string(invalidReconciliationJSON), reconciliation, reconciliation},
	}
	runner := &fakeRunner{pageCount: 10}
	service := New(
		config.ToolConfig{PDFToPPM: "pdftoppm", QPDF: "qpdf"},
		runner,
		provider,
		WithArtifactDir(filepath.Join(dir, "artifacts")),
	)
	req := Request{
		Path:          pdf,
		OCRModel:      "gemini-flash-lite-latest",
		BoundaryModel: "gemini-3.1-flash-lite",
		OCRInputMode:  OCRInputModePDFDirect,
		ReviewID:      "copy-10-pages",
	}
	res, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if provider.documentCalls != 4 || res.APICalls != 4 {
		t.Fatalf("calls: provider=%d response=%d, want two chunks plus one rejected and one valid reconciliation", provider.documentCalls, res.APICalls)
	}
	if !slices.Equal(provider.documentModels, []string{
		"gemini-flash-lite-latest",
		"gemini-flash-lite-latest",
		"gemini-3.1-flash-lite",
		"gemini-3.1-flash-lite",
	}) {
		t.Fatalf("document models = %#v, want chunk model followed by boundary model", provider.documentModels)
	}
	if len(res.Pages) != 10 || len(res.Questions) != 3 || !strings.Contains(res.Report, "Visible question slots: 3") {
		t.Fatalf("response = %#v, want ten pages, three question slots, and deterministic inventory", res)
	}
	questionsByID := make(map[string]Question, len(res.Questions))
	for _, question := range res.Questions {
		questionsByID[question.ID] = question
	}
	second := questionsByID["q2"]
	if !slices.Equal(second.SourcePages, []int{7, 8, 9, 10}) || second.AnswerMarkdown != "complete second answer" {
		t.Fatalf("second question = %#v, want complete canonical overlap candidate on global pages 7-10", second)
	}
	if res.Metadata == nil || res.Metadata.TopperName != "Sample Topper" {
		t.Fatalf("metadata = %#v, want merged chunk metadata", res.Metadata)
	}
	if res.Metadata.Notes != "" {
		t.Fatalf("metadata notes = %q, want chunk-local notes omitted", res.Metadata.Notes)
	}
	third := questionsByID["q3"]
	if third.Status != directPDFQuestionUnanswered || third.AnswerMarkdown != "" || !slices.Equal(third.SourcePages, []int{5}) {
		t.Fatalf("third question = %#v, want explicit unanswered slot", third)
	}

	resumed, err := service.Run(context.Background(), req)
	if err != nil {
		t.Fatalf("resumed Run() error = %v", err)
	}
	if provider.documentCalls != 5 {
		t.Fatalf("provider calls after resume = %d, want only reconciliation rerun", provider.documentCalls)
	}
	if provider.documentModels[4] != "gemini-3.1-flash-lite" {
		t.Fatalf("resumed reconciliation model = %q, want boundary model", provider.documentModels[4])
	}
	if resumed.APICalls != 3 || len(resumed.Questions) != 3 {
		t.Fatalf("resumed response = %#v, want checkpoint call provenance and complete questions", resumed)
	}
	if provider.documentRequest.ResponseSchema != nil || !strings.Contains(provider.documentPrompt, `"inventory"`) {
		t.Fatalf("reconciliation request should carry its schema in the prompt for provider-independent validation")
	}
	if len(provider.documentPrompts) < 4 || !strings.Contains(provider.documentPrompts[3], "inventory mismatch") {
		t.Fatalf("strict reconciliation retry should include the precise semantic validation failure")
	}
	if !strings.Contains(provider.documentPrompts[3], string(invalidReconciliationJSON)) ||
		!strings.Contains(provider.documentPrompts[3], "Return a corrected full JSON object, not a patch") {
		t.Fatalf("strict reconciliation retry should include the rejected full response and request a corrected full plan")
	}
}

func TestQuestionScorecardAcceptsFractionalModelScores(t *testing.T) {
	t.Parallel()

	var scorecard QuestionScorecard
	err := json.Unmarshal([]byte(`{
		"demand_fulfilment":4.5,
		"structure":3.4,
		"content_depth":8,
		"evidence":-1,
		"overall_percent":1e100,
		"estimated_band":"strong",
		"confidence":"high"
	}`), &scorecard)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if scorecard.DemandFulfilment != 5 || scorecard.Structure != 3 || scorecard.ContentDepth != 5 || scorecard.Evidence != 0 || scorecard.OverallPercent != 100 {
		t.Fatalf("scorecard = %#v, want rounded and clamped model scores", scorecard)
	}
}

func TestQuestionMetadataAcceptsOnlyBoundedFiniteNumericMarks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    float64
		wantErr bool
	}{
		{name: "fractional", raw: "12.5", want: 12.5},
		{name: "upper bound", raw: "1000", want: 1000},
		{name: "negative", raw: "-0.5", wantErr: true},
		{name: "above upper bound", raw: "1000.5", wantErr: true},
		{name: "non finite overflow", raw: "1e1000", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var metadata QuestionMetadata
			err := json.Unmarshal([]byte(fmt.Sprintf(`{"marks":%s}`, test.raw)), &metadata)
			if test.wantErr {
				if err == nil {
					t.Fatalf("Unmarshal() error = nil, want bounded-number rejection for %s", test.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if metadata.Marks != test.want {
				t.Fatalf("Marks = %v, want %v", metadata.Marks, test.want)
			}
		})
	}
}

func TestApplyDirectPDFReconciliationRequiresEveryCandidateExactlyOnce(t *testing.T) {
	t.Parallel()

	candidates := []directPDFCandidate{
		{ID: "c1", SourcePages: []int{1}, AnswerMarkdown: "one", question: Question{ID: "q1", Label: "Q.1", SourcePages: []int{1}, AnswerMarkdown: "one"}},
		{ID: "c2", SourcePages: []int{2}, AnswerMarkdown: "two", question: Question{ID: "q2", Label: "Q.2", SourcePages: []int{2}, AnswerMarkdown: "two"}},
	}
	_, _, err := applyDirectPDFReconciliation(candidates, directPDFReconciliation{
		Groups: []directPDFReconciliationGroup{{
			ID:                   "q1",
			Status:               directPDFQuestionAnswered,
			CandidateIDs:         []string{"c1"},
			CanonicalCandidateID: "c1",
			Label:                "Q.1",
			SourcePages:          []int{1},
			Confidence:           1,
			Reason:               "visible answer",
		}},
		Inventory: directPDFReconciliationInventory{VisibleQuestionSlots: 1, Answered: 1},
	}, 2)
	if err == nil || !strings.Contains(err.Error(), `missing candidate IDs=["c2"]`) {
		t.Fatalf("applyDirectPDFReconciliation() error = %v, want strict missing-candidate error", err)
	}
}

func TestApplyDirectPDFReconciliationReportsAllCandidateAssignmentViolations(t *testing.T) {
	t.Parallel()

	candidates := []directPDFCandidate{
		{ID: "c1", SourcePages: []int{1}, AnswerMarkdown: "one", question: Question{ID: "q1", Label: "Q.1", SourcePages: []int{1}, AnswerMarkdown: "one"}},
		{ID: "c2", SourcePages: []int{2}, AnswerMarkdown: "two", question: Question{ID: "q2", Label: "Q.2", SourcePages: []int{2}, AnswerMarkdown: "two"}},
		{ID: "c3", SourcePages: []int{3}, AnswerMarkdown: "three", question: Question{ID: "q3", Label: "Q.3", SourcePages: []int{3}, AnswerMarkdown: "three"}},
	}
	plan := directPDFReconciliation{
		Groups: []directPDFReconciliationGroup{
			{
				ID:                   "slot-one",
				Status:               directPDFQuestionAnswered,
				CandidateIDs:         []string{"c1", " unknown-candidate "},
				CanonicalCandidateID: "c1",
				Label:                "Q.1",
				SourcePages:          []int{1},
				Confidence:           1,
				Reason:               "visible first answer",
			},
			{
				ID:                   "slot-two",
				Status:               directPDFQuestionAnswered,
				CandidateIDs:         []string{" c1 "},
				CanonicalCandidateID: " c1 ",
				Label:                "Q.2",
				SourcePages:          []int{2},
				Confidence:           1,
				Reason:               "candidate was assigned here too",
			},
		},
		Inventory: directPDFReconciliationInventory{VisibleQuestionSlots: 2, Answered: 2},
	}

	_, _, err := applyDirectPDFReconciliation(candidates, plan, 3)
	if err == nil {
		t.Fatal("applyDirectPDFReconciliation() error = nil, want complete assignment violation")
	}
	for _, violation := range []string{
		`unknown candidate IDs=["unknown-candidate"]`,
		`duplicated candidate IDs=["c1"]`,
		`missing candidate IDs=["c2" "c3"]`,
	} {
		if !strings.Contains(err.Error(), violation) {
			t.Fatalf("applyDirectPDFReconciliation() error = %v, want violation %q", err, violation)
		}
	}
}

func TestApplyDirectPDFReconciliationRequiresSemanticMergeForDisjointContinuations(t *testing.T) {
	t.Parallel()

	candidates := []directPDFCandidate{
		{ID: "c1", SourcePages: []int{7, 8}, AnswerMarkdown: "start", question: Question{ID: "q2", Label: "Q.2", SourcePages: []int{7, 8}, AnswerMarkdown: "start"}},
		{ID: "c2", SourcePages: []int{9, 10}, AnswerMarkdown: "end", question: Question{ID: "continuation", Label: "Continuation", SourcePages: []int{9, 10}, AnswerMarkdown: "end"}},
	}
	plan := directPDFReconciliation{
		Groups: []directPDFReconciliationGroup{{
			ID:                   "q2",
			Status:               directPDFQuestionAnswered,
			CandidateIDs:         []string{"c1", "c2"},
			CanonicalCandidateID: "c1",
			Label:                "Q.2",
			SourcePages:          []int{7, 8, 9, 10},
			Confidence:           0.95,
			Reason:               "semantic continuation",
		}},
		Inventory: directPDFReconciliationInventory{VisibleQuestionSlots: 1, Answered: 1},
	}
	if _, _, err := applyDirectPDFReconciliation(candidates, plan, 10); err == nil || !strings.Contains(err.Error(), "no merged answer") {
		t.Fatalf("applyDirectPDFReconciliation() error = %v, want merge-required error", err)
	}
	plan.Groups[0].MergedAnswerMarkdown = "start\n\nend"
	questions, _, err := applyDirectPDFReconciliation(candidates, plan, 10)
	if err != nil {
		t.Fatalf("applyDirectPDFReconciliation() merged error = %v", err)
	}
	if len(questions) != 1 || !slices.Equal(questions[0].SourcePages, []int{7, 8, 9, 10}) || questions[0].AnswerMarkdown != "start\n\nend" {
		t.Fatalf("questions = %#v, want one semantically merged continuation", questions)
	}
}

func TestApplyDirectPDFReconciliationPreservesCandidateBackedUnansweredSlot(t *testing.T) {
	t.Parallel()

	candidates := []directPDFCandidate{
		{
			ID:             "c1",
			SourcePages:    []int{1, 2},
			AnswerMarkdown: "visible answer",
			question: Question{
				ID:             "candidate-q1",
				Label:          "Q.1",
				Title:          "Answered prompt",
				SourcePages:    []int{1, 2},
				AnswerMarkdown: "visible answer",
			},
		},
		{
			ID:             "c2",
			SourcePages:    []int{3, 4},
			AnswerMarkdown: "Printed prompt; the visible answer area is blank.",
			question: Question{
				ID:             "candidate-q2",
				Label:          "Q.2",
				Title:          "Unanswered prompt",
				SourcePages:    []int{3, 4},
				AnswerMarkdown: "Printed prompt; the visible answer area is blank.",
				Status:         directPDFQuestionUnanswered,
			},
		},
	}
	plan := directPDFReconciliation{
		Groups: []directPDFReconciliationGroup{
			{
				ID:                   "slot-one",
				Status:               directPDFQuestionAnswered,
				CandidateIDs:         []string{"c1"},
				CanonicalCandidateID: "c1",
				Label:                "Q.1",
				Title:                "Answered prompt",
				SourcePages:          []int{1, 2},
				Confidence:           0.99,
				Reason:               "visible handwritten answer",
			},
			{
				ID:           "slot-two",
				Status:       directPDFQuestionUnanswered,
				CandidateIDs: []string{"c2"},
				Label:        "Q.2",
				Title:        "Unanswered prompt",
				SourcePages:  []int{3, 4},
				Confidence:   0.98,
				Reason:       "printed prompt followed by blank answer pages",
			},
		},
		Inventory: directPDFReconciliationInventory{VisibleQuestionSlots: 2, Answered: 1, Unanswered: 1},
	}
	questions, _, err := applyDirectPDFReconciliation(candidates, plan, 4)
	if err != nil {
		t.Fatalf("applyDirectPDFReconciliation() error = %v", err)
	}
	if len(questions) != 2 || questions[0].ID != "slot-one" || questions[1].ID != "slot-two" {
		t.Fatalf("questions = %#v, want stable unique semantic ids", questions)
	}
	if questions[0].Status != directPDFQuestionAnswered || questions[1].Status != directPDFQuestionUnanswered {
		t.Fatalf("statuses = %q, %q, want answered and unanswered", questions[0].Status, questions[1].Status)
	}
	if questions[1].AnswerMarkdown != "" || questions[1].Dimensions != nil || !slices.Equal(questions[1].SourcePages, []int{3, 4}) {
		t.Fatalf("unanswered question = %#v, want blank content with preserved pages", questions[1])
	}
}

func TestApplyDirectPDFReconciliationSupportsFullyUnansweredCopy(t *testing.T) {
	t.Parallel()

	candidates := []directPDFCandidate{{
		ID:             "blank-candidate",
		SourcePages:    []int{1, 2},
		AnswerMarkdown: "Printed prompt; both visible answer pages are blank.",
		question: Question{
			ID:             "candidate-q1",
			Label:          "Q.1",
			Title:          "Visible prompt",
			SourcePages:    []int{1, 2},
			AnswerMarkdown: "Printed prompt; both visible answer pages are blank.",
			Status:         directPDFQuestionUnanswered,
		},
	}}
	plan := directPDFReconciliation{
		Groups: []directPDFReconciliationGroup{{
			ID:           "blank-slot",
			Status:       directPDFQuestionUnanswered,
			CandidateIDs: []string{"blank-candidate"},
			Label:        "Q.1",
			Title:        "Visible prompt",
			SourcePages:  []int{1, 2},
			Confidence:   1,
			Reason:       "prompt is visible and both answer pages are blank",
		}},
		Inventory: directPDFReconciliationInventory{VisibleQuestionSlots: 1, Unanswered: 1},
	}
	questions, _, err := applyDirectPDFReconciliation(candidates, plan, 2)
	if err != nil {
		t.Fatalf("applyDirectPDFReconciliation() error = %v", err)
	}
	if len(questions) != 1 || questions[0].Status != directPDFQuestionUnanswered || questions[0].AnswerMarkdown != "" {
		t.Fatalf("questions = %#v, want one preserved unanswered slot", questions)
	}
}

func TestApplyDirectPDFReconciliationRejectsInvalidSemanticInventory(t *testing.T) {
	t.Parallel()

	candidate := directPDFCandidate{
		ID:             "c1",
		SourcePages:    []int{1},
		AnswerMarkdown: "answer",
		question:       Question{ID: "q1", Label: "Q.1", SourcePages: []int{1}, AnswerMarkdown: "answer"},
	}
	validPlan := func() directPDFReconciliation {
		return directPDFReconciliation{
			Groups: []directPDFReconciliationGroup{{
				ID:                   "slot-one",
				Status:               directPDFQuestionAnswered,
				CandidateIDs:         []string{"c1"},
				CanonicalCandidateID: "c1",
				Label:                "Q.1",
				SourcePages:          []int{1},
				Confidence:           1,
				Reason:               "visible answer",
			}},
			Inventory: directPDFReconciliationInventory{VisibleQuestionSlots: 1, Answered: 1},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*directPDFReconciliation)
		wantErr string
	}{
		{
			name: "duplicate semantic id",
			mutate: func(plan *directPDFReconciliation) {
				duplicate := plan.Groups[0]
				duplicate.ID = "SLOT-ONE"
				duplicate.Status = directPDFQuestionUnanswered
				duplicate.CandidateIDs = nil
				duplicate.CanonicalCandidateID = ""
				duplicate.SourcePages = []int{2}
				plan.Groups = append(plan.Groups, duplicate)
				plan.Inventory = directPDFReconciliationInventory{VisibleQuestionSlots: 2, Answered: 1, Unanswered: 1}
			},
			wantErr: "not unique",
		},
		{
			name: "out of bounds page",
			mutate: func(plan *directPDFReconciliation) {
				plan.Groups[0].SourcePages = []int{3}
			},
			wantErr: "outside 1-2",
		},
		{
			name: "duplicate source page",
			mutate: func(plan *directPDFReconciliation) {
				plan.Groups[0].SourcePages = []int{1, 1}
			},
			wantErr: "duplicated",
		},
		{
			name: "unanswered group contains generated answer",
			mutate: func(plan *directPDFReconciliation) {
				plan.Groups[0].Status = directPDFQuestionUnanswered
				plan.Groups[0].CanonicalCandidateID = ""
				plan.Groups[0].MergedAnswerMarkdown = "invented"
				plan.Inventory = directPDFReconciliationInventory{VisibleQuestionSlots: 1, Unanswered: 1}
			},
			wantErr: "must not contain an answer",
		},
		{
			name: "unanswered group without candidate evidence",
			mutate: func(plan *directPDFReconciliation) {
				plan.Groups[0].Status = directPDFQuestionUnanswered
				plan.Groups[0].CandidateIDs = nil
				plan.Groups[0].CanonicalCandidateID = ""
				plan.Inventory = directPDFReconciliationInventory{VisibleQuestionSlots: 1, Unanswered: 1}
			},
			wantErr: "has no candidates",
		},
		{
			name: "inventory count mismatch",
			mutate: func(plan *directPDFReconciliation) {
				plan.Inventory = directPDFReconciliationInventory{VisibleQuestionSlots: 2, Answered: 1, Unanswered: 1}
			},
			wantErr: "inventory mismatch",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			plan := validPlan()
			test.mutate(&plan)
			_, _, err := applyDirectPDFReconciliation([]directPDFCandidate{candidate}, plan, 2)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("applyDirectPDFReconciliation() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestBuildDirectPDFReconciliationReportUsesValidatedQuestionInventory(t *testing.T) {
	t.Parallel()

	questions := []Question{
		{ID: "slot-one", Label: "Q.1", Title: "Answered prompt", Status: directPDFQuestionAnswered, SourcePages: []int{1, 2}},
		{ID: "slot-two", Label: "Q.2", Title: "Blank prompt", Status: directPDFQuestionUnanswered, SourcePages: []int{3, 4}},
	}
	plan := directPDFReconciliation{
		Inventory: directPDFReconciliationInventory{VisibleQuestionSlots: 2, Answered: 1, Unanswered: 1},
		Report:    testDirectPDFReconciliationReport("slot-one"),
	}
	report, err := buildDirectPDFReconciliationReport(questions, plan)
	if err != nil {
		t.Fatalf("buildDirectPDFReconciliationReport() error = %v", err)
	}
	for _, fact := range []string{"Visible question slots: 2", "Answered: 1", "Unanswered: 1", "Q.2 (pages 3, 4)", "Status:** Unanswered"} {
		if !strings.Contains(report, fact) {
			t.Fatalf("report missing deterministic fact %q:\n%s", fact, report)
		}
	}
	plan.Report.AnswerAnalyses = nil
	if _, err := buildDirectPDFReconciliationReport(questions, plan); err == nil || !strings.Contains(err.Error(), "omitted answered group") {
		t.Fatalf("missing answer analysis error = %v, want strict coverage error", err)
	}
}

func TestAnalysisQualityExcludesUnansweredSlotsFromAnalysisCoverage(t *testing.T) {
	t.Parallel()

	confidence := 1.0
	quality := analysisQuality(
		[]Page{{Number: 1, Kind: "answer", KindConfidence: 1, OCRConfidence: &confidence, Text: "visible text"}},
		[]Question{
			{
				ID:     "answered",
				Title:  "Answered prompt",
				Status: directPDFQuestionAnswered,
				Dimensions: &QuestionDimensions{Strengths: []AnalysisPoint{{
					Point:    "specific strength",
					Evidence: "visible evidence",
				}}},
			},
			{ID: "unanswered", Title: "Blank prompt", Status: directPDFQuestionUnanswered},
		},
	)
	if quality.AnalysisCoveragePercent != 100 || quality.EvidenceCoveragePercent != 100 || quality.PromptMatchPercent != 100 {
		t.Fatalf("quality = %#v, want unanswered slot excluded only from analysis denominators", quality)
	}
}

func testDirectPDFReconciliationReport(answerGroupIDs ...string) directPDFReconciliationReport {
	answerNotes := make([]directPDFReconciliationAnswerNote, 0, len(answerGroupIDs))
	for _, id := range answerGroupIDs {
		answerNotes = append(answerNotes, directPDFReconciliationAnswerNote{GroupID: id, Analysis: "evidence-based answer analysis"})
	}
	return directPDFReconciliationReport{
		CopyProfile:                   "Qualitative copy profile without inventory arithmetic.",
		ScorecardSynthesis:            "Evidence-grounded scorecard synthesis.",
		AnswerAnalyses:                answerNotes,
		RepeatedWinningPatterns:       "Repeated evidence-grounded patterns.",
		WhatNotToCopyBlindly:          "Specific risks to avoid.",
		GapMap:                        "Demand-relevant gaps.",
		ReusableAnswerWritingPlaybook: "Reusable answer-writing techniques.",
		DeliberatePracticePlan:        "Observable deliberate-practice drills.",
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

	content := "```json\n{\"metadata\":{\"topper_name\":\"A Topper\",\"paper\":\"GS1\"},\"detected_questions\":[\"Q.1\"],\"pages\":[{\"number\":1,\"name\":\"page-1\",\"text\":\"ocr text\",\"unclear_count\":1}],\"questions\":[{\"label\":\"Q.1\",\"title\":\"History\",\"source_pages\":[1],\"answer_markdown\":\"test answer\",\"dimensions\":{\"fact\":\"good examples\",\"strengths\":[{\"point\":\"specific example\",\"evidence\":\"visible case\"}],\"scorecard\":{\"evidence\":4,\"overall_percent\":76,\"estimated_band\":\"strong\",\"confidence\":\"high\"}},\"metadata\":{\"subject\":\"History\",\"topic\":\"Ancient India\",\"marks\":12.5}}],\"report\":\"test report\"}\n```"
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
	if questions[0].Metadata == nil || questions[0].Metadata.Topic != "Ancient India" || questions[0].Metadata.Marks != 12.5 {
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
