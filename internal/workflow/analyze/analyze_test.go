package analyze

import (
	"context"
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
	visionPrompt  string
	chatPrompt    string
	chatPrompts   []string
	chatResponses []string
}

func (fakeProvider) ID() string { return "fake" }
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
	for _, want := range []string{"UPSC topper answer-copy", "diagrams", "marks", "[unclear]"} {
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

func hasArgPair(args []string, key string, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == key && args[i+1] == value {
			return true
		}
	}
	return false
}
