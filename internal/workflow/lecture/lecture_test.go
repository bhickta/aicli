package lecture

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeProvider struct {
	prompt string
}

func (fakeProvider) ID() string { return "fake" }
func (fakeProvider) Health(context.Context) error {
	return nil
}
func (fakeProvider) ListModels(context.Context) ([]provider.Model, error) {
	return nil, nil
}
func (p *fakeProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	p.prompt = req.Messages[0].Content
	return provider.ChatResponse{Content: "# Lecture\n\nGenerated script."}, nil
}
func (fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (fakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

type fakeRunner struct {
	command string
	args    []string
}

func (r *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	r.command = command
	r.args = append([]string(nil), args...)
	output := args[len(args)-1]
	return []byte("ok"), os.WriteFile(output, []byte("audio"), 0o644)
}

func TestRunGeneratesLectureAndAudio(t *testing.T) {
	t.Parallel()

	vault := t.TempDir()
	source := filepath.Join(vault, "zettelkasten")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "economy.md"), []byte("Inflation means sustained rise in prices."), 0o644); err != nil {
		t.Fatal(err)
	}
	provider := &fakeProvider{}
	runner := &fakeRunner{}
	res, err := New(
		provider,
		config.ToolConfig{OTSTTS: "ots.TTS", OTSTTSArgs: `SOAR --input "{script}" --output "{audio}"`},
		runner,
		WithArtifactDir(filepath.Join(t.TempDir(), "artifacts", "lectures")),
	).Run(context.Background(), Request{
		Model:           "local-model",
		VaultPath:       vault,
		SourcePath:      source,
		OutputName:      "Inflation",
		SynthesizeAudio: true,
	}, nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Kind != "lecture" || res.ScriptPath == "" || res.AudioPath == "" {
		t.Fatalf("Response = %#v, want lecture script and audio", res)
	}
	if !strings.Contains(provider.prompt, "Inflation means sustained rise") {
		t.Fatalf("prompt = %q, want source note content", provider.prompt)
	}
	if runner.command != "ots.TTS" {
		t.Fatalf("command = %q, want ots.TTS", runner.command)
	}
	if len(runner.args) != 5 || runner.args[0] != "SOAR" || runner.args[2] != res.ScriptPath || runner.args[4] != res.AudioPath {
		t.Fatalf("args = %#v, want SOAR input/output args", runner.args)
	}
}

func TestCollectNotesRejectsOutsideVault(t *testing.T) {
	t.Parallel()

	vault := t.TempDir()
	outside := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := collectNotes(Request{VaultPath: vault, SourcePath: outside})
	if err == nil {
		t.Fatal("collectNotes() error = nil, want outside vault error")
	}
}

func TestSplitArgsKeepsQuotedPaths(t *testing.T) {
	t.Parallel()

	args, err := splitArgs(`SOAR --input "/tmp/a b/script.md" --output "/tmp/a b/audio.wav"`)
	if err != nil {
		t.Fatalf("splitArgs() error = %v", err)
	}
	if len(args) != 5 || args[2] != "/tmp/a b/script.md" || args[4] != "/tmp/a b/audio.wav" {
		t.Fatalf("args = %#v, want quoted paths preserved", args)
	}
}
