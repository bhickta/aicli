package video

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	progressmodel "github.com/bhickta/aicli/internal/progress"
)

type courseRunner struct {
	mu              sync.Mutex
	calls           []runnerCall
	ffprobeFailures map[string]bool
}

func (r *courseRunner) CombinedOutput(_ context.Context, command string, args ...string) ([]byte, error) {
	r.mu.Lock()
	r.calls = append(r.calls, runnerCall{command: command, args: append([]string(nil), args...)})
	r.mu.Unlock()
	if command == "ffprobe" {
		if len(args) > 0 && r.ffprobeFailures[args[len(args)-1]] {
			return []byte("moov atom not found"), os.ErrInvalid
		}
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

func TestCourseUsesRequestedOutputName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "01 intro.mp4")
	writeCourseVideoWithSRT(t, videoPath)

	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, &courseRunner{}).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", OutputName: "Philosophy Tanu Jain"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	wantVideo := filepath.Join(dir, "Course", "Philosophy Tanu Jain_Slideshow.mp4")
	wantSRT := filepath.Join(dir, "Course", "Philosophy Tanu Jain.srt")
	wantText := filepath.Join(dir, "Course", "Philosophy Tanu Jain.txt")
	if res.VideoPath != wantVideo || res.SRTPath != wantSRT || res.TextPath != wantText {
		t.Fatalf("artifacts = %q, %q, %q; want %q, %q, %q", res.VideoPath, res.SRTPath, res.TextPath, wantVideo, wantSRT, wantText)
	}
	for _, path := range []string{wantVideo, wantSRT, wantText} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
	}
}

func TestCourseTempCompressionSkipsRedundantSubtitleMuxingAndFaststart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "01 intro.mp4")
	writeCourseVideoWithSRT(t, videoPath)

	runner := &courseRunner{}
	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	for _, call := range runner.calls {
		if call.command != "ffmpeg" || len(call.args) == 0 || !strings.Contains(call.args[len(call.args)-1], ".aicli_cache/slideshows") {
			continue
		}
		args := strings.Join(call.args, " ")
		if !strings.Contains(args, "-sn") {
			t.Fatalf("course temp compression args = %q, want subtitle streams disabled", args)
		}
		for _, notWant := range []string{"-map 1:s?", "-map 0:s?", "-movflags +faststart"} {
			if strings.Contains(args, notWant) {
				t.Fatalf("course temp compression args = %q, did not want %q", args, notWant)
			}
		}
		return
	}
	t.Fatalf("no course temp compression ffmpeg call found: %#v", runner.calls)
}

func TestCourseFinalMergeEmbedsSRTInSinglePass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "01 intro.mp4")
	second := filepath.Join(dir, "02 lesson.mp4")
	writeCourseVideoWithSRT(t, first)
	writeCourseVideoWithSRT(t, second)

	runner := &courseRunner{}
	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	finalVideo := filepath.Join(dir, "Course", filepath.Base(dir)+"_Slideshow.mp4")
	finalSRT := filepath.Join(dir, "Course", filepath.Base(dir)+".srt")
	for _, call := range runner.calls {
		if call.command != "ffmpeg" || len(call.args) == 0 {
			continue
		}
		if strings.Contains(strings.Join(call.args, " "), "_tmp.mp4") {
			t.Fatalf("ffmpeg args = %#v, did not want tmp final video pass", call.args)
		}
		if call.args[len(call.args)-1] != finalVideo {
			continue
		}
		args := strings.Join(call.args, " ")
		for _, want := range []string{"-f concat", "-i " + finalSRT, "-map 1:s:0", "-c copy", "-c:s mov_text"} {
			if !strings.Contains(args, want) {
				t.Fatalf("final merge args = %q, want contains %q", args, want)
			}
		}
		return
	}
	t.Fatalf("no final single-pass merge call found: %#v", runner.calls)
}

func TestCourseUsesOptionalWorkDirForCache(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workDir := t.TempDir()
	videoPath := filepath.Join(dir, "01 intro.mp4")
	writeCourseVideoWithSRT(t, videoPath)

	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, &courseRunner{}).Course(
		context.Background(),
		CourseRequest{Path: dir, WorkDir: workDir, Preset: "slideshow", FPS: "1/2"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".aicli_cache")); !os.IsNotExist(err) {
		t.Fatalf("source .aicli_cache stat err = %v, want not exist when work dir is set", err)
	}
	matches, err := filepath.Glob(filepath.Join(workDir, "*", "01 intro.srt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("work dir cache matches = %#v, want one cached transcript", matches)
	}
}

func TestCourseCleanupVerifiedPartsRemovesSourcesAndCacheOnlyAfterExport(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "01 intro.mp4")
	second := filepath.Join(dir, "02 lesson.mp4")
	writeCourseVideoWithSRT(t, first)
	writeCourseVideoWithSRT(t, second)

	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, &courseRunner{}).Course(
		context.Background(),
		CourseRequest{
			Path:            dir,
			Preset:          "slideshow",
			FPS:             "1/2",
			MaxMergeHours:   0.0001,
			CleanupVerified: true,
		},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(dir, "Course", filepath.Base(dir)+"_Part1_Slideshow.mp4"),
		filepath.Join(dir, "Course", filepath.Base(dir)+"_Part1.srt"),
		filepath.Join(dir, "Course", filepath.Base(dir)+"_Part1.txt"),
		filepath.Join(dir, "Course", filepath.Base(dir)+"_Part2_Slideshow.mp4"),
		filepath.Join(dir, "Course", filepath.Base(dir)+"_Part2.srt"),
		filepath.Join(dir, "Course", filepath.Base(dir)+"_Part2.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected verified course artifact %s: %v", path, err)
		}
	}
	for _, path := range []string{
		first,
		second,
		strings.TrimSuffix(first, filepath.Ext(first)) + ".srt",
		strings.TrimSuffix(second, filepath.Ext(second)) + ".srt",
		filepath.Join(dir, ".aicli_cache"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected cleaned path %s to be removed, stat err = %v", path, err)
		}
	}
	if len(res.Cleaned) == 0 {
		t.Fatal("Cleaned is empty, want removed source/cache paths")
	}
}

func TestCourseUsesCachedSRTWithoutCallingWhisper(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "01 intro.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, ".aicli_cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "01 intro.srt"), []byte("1\n00:00:00,000 --> 00:00:01,000\ncached transcript\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &courseRunner{}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe", WhisperCLI: "whisper-cli"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	if len(res.Transcribed) != 0 {
		t.Fatalf("transcribed len = %d, want 0 when cached SRT exists", len(res.Transcribed))
	}
	for _, call := range runner.calls {
		if call.command == "whisper-cli" {
			t.Fatalf("whisper-cli was called despite cached SRT: %#v", runner.calls)
		}
	}
	text, err := os.ReadFile(filepath.Join(cacheDir, "01 intro.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "cached transcript") {
		t.Fatalf("cache text = %q, want cached transcript", string(text))
	}
}

func TestCourseFailsUnreadableVideosByDefault(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goodPath := filepath.Join(dir, "01 intro.mp4")
	badPath := filepath.Join(dir, "02 corrupt.mp4")
	writeCourseVideoWithSRT(t, goodPath)
	if err := os.WriteFile(badPath, []byte("not a real mp4"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &courseRunner{ffprobeFailures: map[string]bool{badPath: true}}
	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2"},
	)
	if err == nil {
		t.Fatal("Course() error = nil, want unreadable video error")
	}
	for _, want := range []string{"Unreadable video", "02 corrupt.mp4", "moov atom not found"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Course() error = %q, want contains %q", err.Error(), want)
		}
	}
}

func TestCourseSkipsUnreadableVideosWhenRequested(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	goodPath := filepath.Join(dir, "01 intro.mp4")
	badPath := filepath.Join(dir, "02 corrupt.mp4")
	writeCourseVideoWithSRT(t, goodPath)
	if err := os.WriteFile(badPath, []byte("not a real mp4"), 0o644); err != nil {
		t.Fatal(err)
	}

	runner := &courseRunner{ffprobeFailures: map[string]bool{badPath: true}}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", SkipUnreadable: true},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	if len(res.Compressed) != 1 {
		t.Fatalf("compressed len = %d, want 1", len(res.Compressed))
	}
	if len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0], "02 corrupt.mp4") {
		t.Fatalf("skipped = %#v, want corrupt video listed", res.Skipped)
	}
}

func TestCourseWorkerSettingsUseSplitValuesWithWorkersFallback(t *testing.T) {
	t.Parallel()

	split := CourseRequest{Workers: 6, TranscriptWorkers: 2, CompressionWorkers: 4}
	if got := courseTranscriptWorkers(split); got != 2 {
		t.Fatalf("courseTranscriptWorkers() = %d, want 2", got)
	}
	if got := courseCompressionWorkers(split); got != 4 {
		t.Fatalf("courseCompressionWorkers() = %d, want 4", got)
	}

	legacy := CourseRequest{Workers: 3}
	if got := courseTranscriptWorkers(legacy); got != 3 {
		t.Fatalf("legacy courseTranscriptWorkers() = %d, want 3", got)
	}
	if got := courseCompressionWorkers(legacy); got != 3 {
		t.Fatalf("legacy courseCompressionWorkers() = %d, want 3", got)
	}
}

func TestCourseWithProgressReportsPerVideoCompletion(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "01 intro.mp4")
	second := filepath.Join(dir, "02 lesson.mp4")
	writeCourseVideoWithSRT(t, first)
	writeCourseVideoWithSRT(t, second)

	type event struct {
		stage   string
		current int
		total   int
	}
	events := []event{}
	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, &courseRunner{}).CourseWithProgress(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", Workers: 1},
		func(update progressmodel.Update) {
			events = append(events, event{stage: update.Stage, current: update.CompletedUnits, total: update.TotalUnits})
		},
	)
	if err != nil {
		t.Fatalf("CourseWithProgress() error = %v", err)
	}
	if len(events) < 4 {
		t.Fatalf("progress events = %#v, want at least 4 events", events)
	}
	if events[0].current != 0 || events[0].total != 5 {
		t.Fatalf("first event = %#v, want 0/5", events[0])
	}
	if events[1].current != 2 || events[1].total != 5 {
		t.Fatalf("second event = %#v, want 2/5", events[1])
	}
	if events[2].current != 4 || events[2].total != 5 {
		t.Fatalf("third event = %#v, want 4/5", events[2])
	}
	if !strings.Contains(events[len(events)-1].stage, "completed") || events[len(events)-1].current != 5 || events[len(events)-1].total != 5 {
		t.Fatalf("last event = %#v, want completed stage at 5/5", events[len(events)-1])
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
