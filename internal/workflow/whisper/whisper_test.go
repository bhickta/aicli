package whisper

import (
	"context"
	"strings"
	"testing"
)

type fakeRunner struct {
	command string
	args    []string
}

func (r *fakeRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	r.command = command
	r.args = append([]string(nil), args...)
	return []byte("ok"), nil
}

func TestRunUsesWhisperCPPArgs(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	_, err := Run(context.Background(), runner, Request{
		Command:    "whisper-cli",
		AudioPath:  "lesson.mp4",
		OutputBase: "/tmp/cache/lesson",
		Model:      "large-v3",
		SRT:        true,
		Text:       true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if runner.command != "whisper-cli" {
		t.Fatalf("command = %q, want whisper-cli", runner.command)
	}
	args := strings.Join(runner.args, " ")
	for _, want := range []string{"-m large-v3", "-f lesson.mp4", "-osrt", "-otxt", "-of /tmp/cache/lesson"} {
		if !strings.Contains(args, want) {
			t.Fatalf("args = %q, want contains %q", args, want)
		}
	}
}

func TestRunUsesPythonWhisperArgs(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	_, err := Run(context.Background(), runner, Request{
		Command:    "whisper",
		AudioPath:  "lesson.mp4",
		OutputBase: "/tmp/cache/lesson",
		Model:      "large-v3",
		Device:     "cuda",
		SRT:        true,
		Text:       true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if runner.command != "whisper" {
		t.Fatalf("command = %q, want whisper", runner.command)
	}
	args := strings.Join(runner.args, " ")
	for _, want := range []string{"lesson.mp4", "--model large-v3", "--device cuda", "--output_format all", "--output_dir /tmp/cache"} {
		if !strings.Contains(args, want) {
			t.Fatalf("args = %q, want contains %q", args, want)
		}
	}
}

func TestRunFasterBatchUsesEmbeddedScriptAndOldFastDefaults(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	_, err := RunFasterBatch(context.Background(), runner, FasterBatchRequest{
		PythonCommand: "python3",
		AudioPaths:    []string{"one.mp4", "two.mp4"},
		OutputDir:     "/tmp/cache",
		Model:         "turbo",
		Device:        "cuda",
		Workers:       2,
	})
	if err != nil {
		t.Fatalf("RunFasterBatch() error = %v", err)
	}
	if runner.command != "python3" {
		t.Fatalf("command = %q, want python3", runner.command)
	}
	args := strings.Join(runner.args, " ")
	for _, want := range []string{
		"-c",
		"from faster_whisper import BatchedInferencePipeline, WhisperModel",
		"--model turbo",
		"--device cuda",
		"--compute-type float16",
		"--workers 2",
		"--batch-size 24",
		"--beam-size 1",
		"--output-dir /tmp/cache",
		"one.mp4 two.mp4",
	} {
		if !strings.Contains(args, want) {
			t.Fatalf("args = %q, want contains %q", args, want)
		}
	}
}
