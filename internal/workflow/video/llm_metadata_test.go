package video

import (
	"context"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

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
