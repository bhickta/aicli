package audio

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct {
	command string
	args    []string
}

func (f *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	f.command = command
	f.args = args
	return []byte("ok"), nil
}

func TestTranscribeReadsWhisperTextOutput(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(audioPath, []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "track.txt"), []byte("transcript"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{}
	res, err := New(config.ToolConfig{WhisperCLI: "whisper-cli"}, runner).Transcribe(
		context.Background(),
		TranscribeRequest{Path: audioPath, Model: "model.bin"},
	)
	if err != nil {
		t.Fatalf("Transcribe() error = %v", err)
	}
	if res.Text != "transcript" {
		t.Fatalf("Text = %q, want transcript", res.Text)
	}
	if runner.command != "whisper-cli" {
		t.Fatalf("command = %q, want whisper-cli", runner.command)
	}
}

type fakeProvider struct {
	calls int
}

func (f *fakeProvider) ID() string { return "fake" }
func (f *fakeProvider) Health(context.Context) error {
	return nil
}
func (f *fakeProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (f *fakeProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	f.calls++
	return provider.ChatResponse{Content: "ok"}, nil
}
func (f *fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (f *fakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

func TestAnalyzeBuildsAnalysisAndPlaylists(t *testing.T) {
	t.Parallel()

	fp := &fakeProvider{}
	res, err := New(config.ToolConfig{}, &fakeRunner{}, fp).Analyze(
		context.Background(),
		AnalyzeRequest{TrackText: []string{"hello"}},
	)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if fp.calls != 2 || res.Analysis == "" || res.Playlists == "" {
		t.Fatalf("calls=%d response=%#v, want analysis and playlists", fp.calls, res)
	}
}
