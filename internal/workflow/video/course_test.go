package video

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

type courseRunner struct {
	calls []runnerCall
}

func (r *courseRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, runnerCall{command: command, args: append([]string(nil), args...)})
	if command == "ffprobe" {
		return []byte("2.0\n"), nil
	}
	if command == "whisper-cli" {
		return writeWhisperOutputs(args)
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

func writeWhisperOutputs(args []string) ([]byte, error) {
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
	writeCourseVideoWithSRT(t, first)
	writeCourseVideoWithSRT(t, second)

	runner := &courseRunner{}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	assertCourseArtifacts(t, dir, res)
	if len(res.Compressed) != 2 {
		t.Fatalf("compressed len = %d, want 2", len(res.Compressed))
	}
	assertNVENCUsed(t, runner.calls)
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
	assertWhisperLargeV3(t, runner.calls)
	mergedText, err := os.ReadFile(filepath.Join(dir, "Course", filepath.Base(dir)+".txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mergedText), "whisper transcript") {
		t.Fatalf("merged text = %q, want whisper transcript", string(mergedText))
	}
}

func writeCourseVideoWithSRT(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	srt := strings.TrimSuffix(path, filepath.Ext(path)) + ".srt"
	if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello "+filepath.Base(path)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertCourseArtifacts(t *testing.T, dir string, res CourseResponse) {
	t.Helper()
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
}

func assertNVENCUsed(t *testing.T, calls []runnerCall) {
	t.Helper()
	for _, call := range calls {
		if call.command == "ffmpeg" && strings.Contains(strings.Join(call.args, " "), "h264_nvenc") {
			return
		}
	}
	t.Fatalf("course did not use NVENC compression calls: %#v", calls)
}

func assertWhisperLargeV3(t *testing.T, calls []runnerCall) {
	t.Helper()
	for i := range calls {
		if calls[i].command != "whisper-cli" {
			continue
		}
		args := strings.Join(calls[i].args, " ")
		for _, want := range []string{"-m large-v3", "-osrt", "-otxt"} {
			if !strings.Contains(args, want) {
				t.Fatalf("whisper args = %q, want contains %q", args, want)
			}
		}
		return
	}
	t.Fatal("whisper-cli was not called")
}
