package video

import (
	"context"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/config"
)

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
	for _, want := range []string{"-hwaccel cuda", "-vf scale_cuda=-2:240", "-c:v h264_nvenc", "-cq 30", "-map 0:s?", "-movflags +faststart"} {
		if !strings.Contains(args, want) {
			t.Fatalf("ffmpeg args = %q, want contains %q", args, want)
		}
	}
}
