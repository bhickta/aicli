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
	for _, want := range []string{"lesson.mp4", "--model large-v3", "--output_format all", "--output_dir /tmp/cache"} {
		if !strings.Contains(args, want) {
			t.Fatalf("args = %q, want contains %q", args, want)
		}
	}
}
