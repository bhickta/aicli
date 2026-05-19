package video

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
	command string
	args    []string
	out     []byte
	err     error
	calls   []runnerCall
}

type runnerCall struct {
	command string
	args    []string
}

func (f *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	f.command = command
	f.args = args
	f.calls = append(f.calls, runnerCall{command: command, args: append([]string(nil), args...)})
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
	if res.Output != "video_240p.mp4" {
		t.Fatalf("Output = %q, want video_240p.mp4", res.Output)
	}
	args := strings.Join(runner.args, " ")
	for _, want := range []string{"-hwaccel cuda", "-vf scale_cuda=-2:240", "-c:v h264_nvenc", "-cq 30"} {
		if !strings.Contains(args, want) {
			t.Fatalf("ffmpeg args = %q, want contains %q", args, want)
		}
	}
}

type courseRunner struct {
	calls []runnerCall
}

func (r *courseRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, runnerCall{command: command, args: append([]string(nil), args...)})
	if command == "ffprobe" {
		return []byte("2.0\n"), nil
	}
	if command == "whisper-cli" {
		outputBase := argAfter(args, "-of")
		if outputBase == "" {
			return []byte("missing -of"), os.ErrInvalid
		}
		if err := os.MkdirAll(filepath.Dir(outputBase), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(outputBase+".srt", []byte("1\n00:00:00,000 --> 00:00:01,000\nwhisper transcript\n"), 0o644); err != nil {
			return nil, err
		}
		if err := os.WriteFile(outputBase+".txt", []byte("whisper transcript"), 0o644); err != nil {
			return nil, err
		}
		return []byte("ok"), nil
	}
	if len(args) > 0 {
		outPath := args[len(args)-1]
		if filepath.IsAbs(outPath) || strings.HasSuffix(outPath, ".mp4") {
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(outPath, []byte("video"), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return []byte("ok"), nil
}

func argAfter(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}

func TestCourseCompressesFolderAndExportsMergedArtifacts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "01 intro.mp4")
	second := filepath.Join(dir, "02 lesson.mp4")
	for _, path := range []string{first, second} {
		if err := os.WriteFile(path, []byte("video"), 0o644); err != nil {
			t.Fatal(err)
		}
		srt := strings.TrimSuffix(path, filepath.Ext(path)) + ".srt"
		if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello "+filepath.Base(path)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runner := &courseRunner{}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	wantVideo := filepath.Join(dir, "Course", filepath.Base(dir)+"_Slideshow.mp4")
	wantSRT := filepath.Join(dir, "Course", filepath.Base(dir)+".srt")
	wantText := filepath.Join(dir, "Course", filepath.Base(dir)+".txt")
	if res.VideoPath != wantVideo {
		t.Fatalf("VideoPath = %q, want %q", res.VideoPath, wantVideo)
	}
	for _, path := range []string{wantVideo, wantSRT, wantText} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}
	srt, err := os.ReadFile(wantSRT)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srt), "00:00:02,000 --> 00:00:03,000") {
		t.Fatalf("merged srt did not shift second transcript by first video duration:\n%s", string(srt))
	}
	text, err := os.ReadFile(wantText)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "--- Segment: 01 intro ---") || !strings.Contains(string(text), "hello 02 lesson.mp4") {
		t.Fatalf("merged transcript text missing expected segments:\n%s", string(text))
	}
	if len(res.Compressed) != 2 {
		t.Fatalf("compressed len = %d, want 2", len(res.Compressed))
	}
	foundNVENC := false
	for _, call := range runner.calls {
		if call.command == "ffmpeg" && strings.Contains(strings.Join(call.args, " "), "h264_nvenc") {
			foundNVENC = true
			break
		}
	}
	if !foundNVENC {
		t.Fatalf("course did not use NVENC compression calls: %#v", runner.calls)
	}
}

func TestCourseTranscribesMissingSRTWithWhisperLargeV3(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "01 intro.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &courseRunner{}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe", WhisperCLI: "whisper-cli"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", WhisperModel: "large-v3"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	if len(res.Transcribed) != 1 {
		t.Fatalf("transcribed len = %d, want 1", len(res.Transcribed))
	}
	var whisperCall *runnerCall
	for i := range runner.calls {
		if runner.calls[i].command == "whisper-cli" {
			whisperCall = &runner.calls[i]
			break
		}
	}
	if whisperCall == nil {
		t.Fatal("whisper-cli was not called")
	}
	args := strings.Join(whisperCall.args, " ")
	for _, want := range []string{"-m large-v3", "-osrt", "-otxt"} {
		if !strings.Contains(args, want) {
			t.Fatalf("whisper args = %q, want contains %q", args, want)
		}
	}
	mergedText, err := os.ReadFile(filepath.Join(dir, "Course", filepath.Base(dir)+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mergedText), "whisper transcript") {
		t.Fatalf("merged text = %q, want whisper transcript", string(mergedText))
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
