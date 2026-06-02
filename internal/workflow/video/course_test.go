package video

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bhickta/aicli/internal/config"
	progressmodel "github.com/bhickta/aicli/internal/progress"
)

type courseRunner struct {
	mu              sync.Mutex
	calls           []runnerCall
	ffprobeFailures map[string]bool
	failOnePassSRT  bool
	failShutdown    bool
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
	if command == "systemctl" && hasArg(args, "poweroff") {
		if r.failShutdown {
			return []byte("permission denied"), os.ErrPermission
		}
		return []byte("ok"), nil
	}
	if command == "ffmpeg" && r.failOnePassSRT && hasArg(args, "-f") && hasArg(args, "concat") && hasArg(args, "-map") && hasArg(args, "1:s:0") {
		return []byte("Packet duration is out of range"), os.ErrInvalid
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

func hasArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
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

func TestCourseShutdownOnCompleteRequestsPoweroffAfterSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "01 intro.mp4")
	writeCourseVideoWithSRT(t, videoPath)

	runner := &courseRunner{}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", ShutdownOnComplete: true},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	if !res.ShutdownRequested {
		t.Fatalf("Shutdown = false, want true")
	}
	for _, call := range runner.calls {
		if call.command == "systemctl" && strings.Join(call.args, " ") == "poweroff" {
			return
		}
	}
	t.Fatalf("no systemctl poweroff call found: %#v", runner.calls)
}

func TestCourseShutdownOnCompleteDoesNotRunAfterCourseFailure(t *testing.T) {
	t.Parallel()

	runner := &courseRunner{}
	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: t.TempDir(), ShutdownOnComplete: true},
	)
	if err == nil {
		t.Fatal("Course() error = nil, want failure")
	}
	for _, call := range runner.calls {
		if call.command == "systemctl" {
			t.Fatalf("shutdown ran after failed course: %#v", runner.calls)
		}
	}
}

func TestCourseShutdownOnCompleteReportsPoweroffFailure(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "01 intro.mp4")
	writeCourseVideoWithSRT(t, videoPath)

	runner := &courseRunner{failShutdown: true}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", ShutdownOnComplete: true},
	)
	if err == nil {
		t.Fatal("Course() error = nil, want shutdown failure")
	}
	if !strings.Contains(err.Error(), "course completed, but shutdown request failed") {
		t.Fatalf("Course() error = %v, want shutdown failure context", err)
	}
	if !res.ShutdownRequested {
		t.Fatalf("Shutdown = false, want true")
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
		for _, want := range []string{"-fflags +genpts", "-f concat", "-i " + finalSRT, "-map 1:s:0", "-avoid_negative_ts make_zero", "-c copy", "-c:s mov_text"} {
			if !strings.Contains(args, want) {
				t.Fatalf("final merge args = %q, want contains %q", args, want)
			}
		}
		return
	}
	t.Fatalf("no final single-pass merge call found: %#v", runner.calls)
}

func TestCourseFinalMergeFallsBackWhenSinglePassFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "01 intro.mp4")
	second := filepath.Join(dir, "02 lesson.mp4")
	writeCourseVideoWithSRT(t, first)
	writeCourseVideoWithSRT(t, second)

	runner := &courseRunner{failOnePassSRT: true}
	res, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner).Course(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2"},
	)
	if err != nil {
		t.Fatalf("Course() error = %v", err)
	}
	assertCourseArtifacts(t, dir, res)

	finalVideo := filepath.Join(dir, "Course", filepath.Base(dir)+"_Slideshow.mp4")
	tmpVideo := courseTmpVideoPath(finalVideo)
	sawFailedOnePass := false
	sawFallbackConcat := false
	sawFallbackEmbed := false
	sawFallbackTimestampRepair := false
	for _, call := range runner.calls {
		if call.command != "ffmpeg" || len(call.args) == 0 {
			continue
		}
		args := strings.Join(call.args, " ")
		switch {
		case call.args[len(call.args)-1] == finalVideo && strings.Contains(args, "-f concat") && strings.Contains(args, "-map 1:s:0"):
			sawFailedOnePass = true
		case call.args[len(call.args)-1] == tmpVideo && strings.Contains(args, "-f concat"):
			sawFallbackConcat = true
			sawFallbackTimestampRepair = strings.Contains(args, "-fflags +genpts") && strings.Contains(args, "-avoid_negative_ts make_zero")
		case call.args[len(call.args)-1] == finalVideo && strings.Contains(args, tmpVideo):
			sawFallbackEmbed = true
		}
	}
	if !sawFailedOnePass || !sawFallbackConcat || !sawFallbackEmbed || !sawFallbackTimestampRepair {
		t.Fatalf("fallback calls missing: onepass=%v concat=%v embed=%v repair=%v calls=%#v", sawFailedOnePass, sawFallbackConcat, sawFallbackEmbed, sawFallbackTimestampRepair, runner.calls)
	}
	if fileExists(tmpVideo) {
		t.Fatalf("temporary fallback video was not cleaned up: %s", tmpVideo)
	}
}

func TestMergeSRTsNormalizesBlocksWhenDurationProbeFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	firstSRT := filepath.Join(dir, "first.srt")
	secondSRT := filepath.Join(dir, "second.srt")
	if err := os.WriteFile(firstSRT, []byte("1\n00:00:00,000 --> 00:00:03,000\nfirst\n\n2\n00:00:20,000 --> 00:05:20,000\nlong block\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondSRT, []byte("1\n00:00:01,000 --> 00:00:04,000\nsecond\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	firstOutput := filepath.Join(dir, "first.mp4")
	secondOutput := filepath.Join(dir, "second.mp4")
	runner := &courseRunner{ffprobeFailures: map[string]bool{firstOutput: true, secondOutput: true}}
	svc := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, runner)
	output := filepath.Join(dir, "merged.srt")
	err := svc.mergeSRTs(context.Background(), []CourseItem{
		{Output: firstOutput, SRTPath: firstSRT},
		{Output: secondOutput, SRTPath: secondSRT},
	}, output)
	if err != nil {
		t.Fatalf("mergeSRTs() error = %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	merged := string(data)
	for _, want := range []string{
		"00:00:20,000 --> 00:02:20,000",
		"00:02:21,000 --> 00:02:24,000",
		"long block",
		"second",
	} {
		if !strings.Contains(merged, want) {
			t.Fatalf("merged SRT = %q, want contains %q", merged, want)
		}
	}
	if strings.Contains(merged, "00:05:20,000") {
		t.Fatalf("merged SRT kept pathological subtitle duration: %q", merged)
	}
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

func TestPlanCoursePartsResumeFillsMissingPartBeforeAppending(t *testing.T) {
	t.Parallel()

	courseDir := t.TempDir()
	folderName := "philosophy"
	for _, existing := range []struct {
		number   int
		segments []string
	}{
		{number: 1, segments: []string{"001 Intro"}},
		{number: 2, segments: []string{"002 Hume"}},
		{number: 3, segments: []string{"003 Kant"}},
		{number: 5, segments: []string{"005 Hegel"}},
		{number: 6, segments: []string{"006 Moore"}},
		{number: 7, segments: []string{"007 Russell"}},
		{number: 8, segments: []string{"008 Wittgenstein"}},
	} {
		writeVerifiedCoursePart(t, courseDir, folderName, existing.number, existing.segments)
	}

	items := []CourseItem{
		{Source: filepath.Join(courseDir, "004 missing.mp4"), TargetName: "004 Missing"},
		{Source: filepath.Join(courseDir, "009 next.mp4"), TargetName: "009 Next"},
	}
	durations := map[string]float64{
		items[0].Source: 2,
		items[1].Source: 2,
	}

	parts := planCourseParts(
		courseDir,
		courseDir,
		CourseRequest{OutputName: folderName, CleanupVerified: true, MaxMergeHours: 9},
		items,
		durations,
	)

	got := coursePartVideoBasenames(parts)
	want := []string{
		"philosophy_Part4_Slideshow.mp4",
		"philosophy_Part9_Slideshow.mp4",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("planned parts = %#v, want %#v", got, want)
	}
}

func TestExportReadyCoursePartsWaitsForEarlierUnreadyPart(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	courseDir := filepath.Join(dir, "Course")
	if err := os.MkdirAll(courseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	first := writeReadyCourseItem(t, dir, "01")
	second := writeReadyCourseItem(t, dir, "02")
	firstArtifact := courseArtifactPaths(courseDir, "course", true, 0)
	secondArtifact := courseArtifactPaths(courseDir, "course", true, 1)
	var completed atomic.Int64
	var firstErr error
	state := pipelineAggregation{
		items: []CourseItem{first, second},
		parts: []plannedCoursePart{
			{
				indexes:  []int{0},
				items:    []CourseItem{first},
				artifact: firstArtifact,
			},
			{
				indexes:  []int{1},
				items:    []CourseItem{second},
				artifact: secondArtifact,
			},
		},
		durations: map[string]float64{
			first.Source:  2,
			second.Source: 2,
		},
		totalUnits:     10,
		completedUnits: &completed,
		response:       &CourseResponse{},
		setErr: func(err error) {
			firstErr = err
		},
	}
	ready := []bool{false, true}
	exported := []bool{false, false}
	svc := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, &courseRunner{})

	svc.exportReadyCourseParts(context.Background(), state, ready, exported)

	if firstErr != nil {
		t.Fatalf("exportReadyCourseParts() error = %v", firstErr)
	}
	if exported[1] {
		t.Fatal("second part exported before first part was ready")
	}
	if fileExists(secondArtifact.videoPath) {
		t.Fatalf("second artifact exists before first part is ready: %s", secondArtifact.videoPath)
	}

	ready[0] = true
	svc.exportReadyCourseParts(context.Background(), state, ready, exported)

	if firstErr != nil {
		t.Fatalf("exportReadyCourseParts() error = %v", firstErr)
	}
	if !exported[0] || !exported[1] {
		t.Fatalf("exported = %#v, want both parts exported after first part is ready", exported)
	}
	for _, path := range []string{
		firstArtifact.videoPath,
		firstArtifact.srtPath,
		firstArtifact.textPath,
		secondArtifact.videoPath,
		secondArtifact.srtPath,
		secondArtifact.textPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected artifact %s: %v", path, err)
		}
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
		label   string
	}
	events := []event{}
	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, &courseRunner{}).CourseWithProgress(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", Workers: 1},
		func(update progressmodel.Update) {
			events = append(events, event{stage: update.Stage, current: update.CompletedUnits, total: update.TotalUnits, label: update.UnitLabel})
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
	for _, event := range events {
		if event.label == "video second" {
			t.Fatalf("course progress event used misleading label: %#v", event)
		}
	}
}

func TestCourseWithProgressDoesNotReportFullCompletionBeforeCourseCompleted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	first := filepath.Join(dir, "01 intro.mp4")
	second := filepath.Join(dir, "02 lesson.mp4")
	writeCourseVideoWithSRT(t, first)
	writeCourseVideoWithSRT(t, second)

	_, err := New(config.ToolConfig{FFmpeg: "ffmpeg", FFprobe: "ffprobe"}, &courseRunner{}).CourseWithProgress(
		context.Background(),
		CourseRequest{Path: dir, Preset: "slideshow", FPS: "1/2", MaxMergeHours: 0.0001, Workers: 1},
		func(update progressmodel.Update) {
			if !strings.Contains(update.Stage, "completed course") && update.TotalUnits > 0 && update.CompletedUnits >= update.TotalUnits {
				t.Fatalf("intermediate progress reported full completion: %#v", update)
			}
		},
	)
	if err != nil {
		t.Fatalf("CourseWithProgress() error = %v", err)
	}
}

func TestFinalizePipelineMissingTranscriptsSkipsAlreadyQueuedItems(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "01 intro.mp4")
	if err := os.WriteFile(source, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, ".aicli_cache")

	called := false
	err := finalizePipelineMissingTranscripts(
		[]pipelineCourseItem{{index: 3, item: CourseItem{Source: source}}},
		cacheDir,
		func(index int) bool { return index == 3 },
		func(int, bool) { called = true },
	)
	if err != nil {
		t.Fatalf("finalizePipelineMissingTranscripts() error = %v, want nil for already queued item", err)
	}
	if called {
		t.Fatal("enqueue called for already queued item")
	}
}

func TestFinalizePipelineMissingTranscriptsRequiresCacheForUnqueuedItems(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	source := filepath.Join(dir, "01 intro.mp4")
	if err := os.WriteFile(source, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := finalizePipelineMissingTranscripts(
		[]pipelineCourseItem{{index: 3, item: CourseItem{Source: source}}},
		filepath.Join(dir, ".aicli_cache"),
		func(int) bool { return false },
		func(int, bool) { t.Fatal("enqueue called without transcript cache") },
	)
	if err == nil || !strings.Contains(err.Error(), "did not produce both .srt and .txt") {
		t.Fatalf("finalizePipelineMissingTranscripts() error = %v, want missing transcript cache error", err)
	}
}

func writeVerifiedCoursePart(t *testing.T, courseDir string, folderName string, number int, segments []string) {
	t.Helper()
	artifact := courseArtifactPaths(courseDir, folderName, true, number-1)
	if err := os.MkdirAll(filepath.Dir(artifact.videoPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact.videoPath, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifact.srtPath, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines := []string{}
	for _, segment := range segments {
		lines = append(lines, "--- Segment: "+segment+" ---\nhello")
	}
	if err := os.WriteFile(artifact.textPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeReadyCourseItem(t *testing.T, dir string, name string) CourseItem {
	t.Helper()
	source := filepath.Join(dir, name+".mp4")
	output := filepath.Join(dir, ".aicli_cache", "slideshows", name+"_slideshow.mp4")
	srt := filepath.Join(dir, ".aicli_cache", name+".srt")
	text := filepath.Join(dir, ".aicli_cache", name+".txt")
	for _, path := range []string{source, output, srt, text} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nhello "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(text, []byte("hello "+name), 0o644); err != nil {
		t.Fatal(err)
	}
	return CourseItem{
		Source:     source,
		Output:     output,
		SRTPath:    srt,
		TextPath:   text,
		TargetName: name,
	}
}

func coursePartVideoBasenames(parts []plannedCoursePart) []string {
	names := make([]string, len(parts))
	for i, part := range parts {
		names[i] = filepath.Base(part.artifact.videoPath)
	}
	return names
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
