package video

import (
	"context"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
)

type fakeRunner struct {
	command string
	args    []string
	out     []byte
	err     error
}

func (f *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	f.command = command
	f.args = args
	return f.out, f.err
}

func TestInfoUsesFFprobeJSON(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{out: []byte(`{"streams":[{"codec_type":"video","codec_name":"h264"}],"format":{"duration":"1.0"}}`)}
	res, err := New(config.ToolConfig{FFprobe: "ffprobe"}, runner).Info(context.Background(), InfoRequest{Path: "video.mp4"})
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if runner.command != "ffprobe" {
		t.Fatalf("command = %q, want ffprobe", runner.command)
	}
	if res.Summary == "" {
		t.Fatal("Summary is empty")
	}
}

func TestCompressUsesFFmpeg(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{out: []byte("ok")}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg"}, runner).Compress(
		context.Background(),
		CompressRequest{Path: "video.mov", CRF: 30},
	)
	if err != nil {
		t.Fatalf("Compress() error = %v", err)
	}
	if runner.command != "ffmpeg" {
		t.Fatalf("command = %q, want ffmpeg", runner.command)
	}
	if res.Output != "video.compressed.mp4" {
		t.Fatalf("Output = %q, want video.compressed.mp4", res.Output)
	}
}

type fakeProvider struct{}

func (fakeProvider) ID() string { return "fake" }
func (fakeProvider) Health(context.Context) error {
	return nil
}
func (fakeProvider) ListModels(context.Context) ([]provider.Model, error) {
	return []provider.Model{}, nil
}
func (fakeProvider) Chat(context.Context, provider.ChatRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{Content: "notes"}, nil
}
func (fakeProvider) ChatStream(context.Context, provider.ChatRequest, func(string) error) error {
	return nil
}
func (fakeProvider) Vision(context.Context, provider.VisionRequest) (provider.ChatResponse, error) {
	return provider.ChatResponse{}, nil
}

func TestGenerateVideoNotes(t *testing.T) {
	t.Parallel()

	res, err := New(config.ToolConfig{}, &fakeRunner{}, fakeProvider{}).Generate(
		context.Background(),
		LLMRequest{Transcript: "lesson", Mode: "notes"},
	)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if res.Text != "notes" {
		t.Fatalf("Text = %q, want notes", res.Text)
	}
}

func TestBackupAndRestoreMetadataUseFFmpeg(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{out: []byte("ok")}
	svc := New(config.ToolConfig{FFmpeg: "ffmpeg"}, runner)
	backup, err := svc.BackupMetadata(context.Background(), MetadataRequest{Path: "video.mp4"})
	if err != nil {
		t.Fatalf("BackupMetadata() error = %v", err)
	}
	if backup.Sidecar != "video.mp4.ffmetadata" {
		t.Fatalf("Sidecar = %q, want video.mp4.ffmetadata", backup.Sidecar)
	}
	restore, err := svc.RestoreMetadata(context.Background(), MetadataRequest{Path: "video.mp4", Sidecar: backup.Sidecar})
	if err != nil {
		t.Fatalf("RestoreMetadata() error = %v", err)
	}
	if restore.Output != "video.restored.mp4" {
		t.Fatalf("Output = %q, want video.restored.mp4", restore.Output)
	}
}
