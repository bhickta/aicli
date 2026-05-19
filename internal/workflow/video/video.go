package video

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
)

type Service struct {
	tools    config.ToolConfig
	runner   tool.Runner
	provider provider.Provider
}

type InfoRequest struct {
	Path string `json:"path"`
}

type InfoResponse struct {
	Raw     json.RawMessage `json:"raw"`
	Summary string          `json:"summary"`
}

type CompressRequest struct {
	Path        string `json:"path"`
	Output      string `json:"output"`
	CRF         int    `json:"crf"`
	Resolution  int    `json:"resolution"`
	Preset      string `json:"preset"`
	Overwrite   bool   `json:"overwrite"`
	FPS         string `json:"fps"`
	FastSkip    bool   `json:"fast_skip"`
	ExternalSRT string `json:"external_srt"`
	TargetName  string `json:"target_name"`
}

type CompressResponse struct {
	Output string `json:"output"`
}

type CourseRequest struct {
	Path          string  `json:"path"`
	OutputDir     string  `json:"output_dir"`
	WhisperModel  string  `json:"whisper_model"`
	Resolution    int     `json:"resolution"`
	Preset        string  `json:"preset"`
	CRF           int     `json:"crf"`
	FPS           string  `json:"fps"`
	FastSkip      bool    `json:"fast_skip"`
	Workers       int     `json:"workers"`
	MaxMergeHours float64 `json:"max_merge_hours"`
}

type CourseResponse struct {
	CourseDir   string       `json:"course_dir"`
	VideoPath   string       `json:"video_path,omitempty"`
	SRTPath     string       `json:"srt_path,omitempty"`
	TextPath    string       `json:"text_path,omitempty"`
	Compressed  []CourseItem `json:"compressed"`
	Transcribed []CourseItem `json:"transcribed,omitempty"`
	Skipped     []string     `json:"skipped,omitempty"`
}

type CourseItem struct {
	Source     string `json:"source"`
	Output     string `json:"output"`
	SRTPath    string `json:"srt_path,omitempty"`
	TextPath   string `json:"text_path,omitempty"`
	TargetName string `json:"target_name"`
}

type MetadataRequest struct {
	Path    string `json:"path"`
	Sidecar string `json:"sidecar"`
	Output  string `json:"output"`
}

type MetadataResponse struct {
	Sidecar string `json:"sidecar,omitempty"`
	Output  string `json:"output,omitempty"`
}

type LLMRequest struct {
	Model      string `json:"model"`
	Title      string `json:"title"`
	Transcript string `json:"transcript"`
	Mode       string `json:"mode"`
}

type LLMResponse struct {
	Text string `json:"text"`
}

func New(tools config.ToolConfig, runner tool.Runner, providers ...provider.Provider) *Service {
	var p provider.Provider
	if len(providers) > 0 {
		p = providers[0]
	}
	return &Service{tools: tools, runner: runner, provider: p}
}

func (s *Service) Info(ctx context.Context, req InfoRequest) (InfoResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return InfoResponse{}, errors.New("path is required")
	}
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFprobe,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		req.Path,
	)
	if err != nil {
		return InfoResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	if !json.Valid(out) {
		return InfoResponse{}, errors.New("ffprobe did not return valid JSON")
	}
	return InfoResponse{Raw: out, Summary: summarize(out)}, nil
}

func (s *Service) Compress(ctx context.Context, req CompressRequest) (CompressResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return CompressResponse{}, errors.New("path is required")
	}
	output, out, err := s.compress(ctx, req)
	if err != nil {
		return CompressResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return CompressResponse{Output: output}, nil
}

func (s *Service) Course(ctx context.Context, req CourseRequest) (CourseResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return CourseResponse{}, errors.New("path is required")
	}
	source, err := filepath.Abs(req.Path)
	if err != nil {
		return CourseResponse{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return CourseResponse{}, err
	}
	targetDir := source
	if !info.IsDir() {
		targetDir = filepath.Dir(source)
	}
	files, err := rawVideoFiles(source)
	if err != nil {
		return CourseResponse{}, err
	}
	if len(files) == 0 {
		return CourseResponse{}, errors.New("no video files found")
	}
	courseDir := req.OutputDir
	if strings.TrimSpace(courseDir) == "" {
		courseDir = filepath.Join(targetDir, "Course")
	}
	if err := os.MkdirAll(courseDir, 0o755); err != nil {
		return CourseResponse{}, err
	}
	cacheDir := filepath.Join(targetDir, ".aicli_cache")
	slidesDir := filepath.Join(cacheDir, "slideshows")
	if err := os.MkdirAll(slidesDir, 0o755); err != nil {
		return CourseResponse{}, err
	}

	items := make([]CourseItem, 0, len(files))
	transcribed := []CourseItem{}
	skipped := []string{}
	usedNames := map[string]int{}
	for _, file := range files {
		targetName := courseTargetName(file, usedNames)
		srtPath, textPath, didTranscribe, prepErr := s.prepareTranscriptFiles(ctx, file, cacheDir, req.WhisperModel)
		if prepErr != nil {
			return CourseResponse{}, prepErr
		}
		if didTranscribe {
			transcribed = append(transcribed, CourseItem{Source: file, SRTPath: srtPath, TextPath: textPath, TargetName: targetName})
		}
		output := filepath.Join(slidesDir, targetName+"_slideshow.mp4")
		if _, statErr := os.Stat(output); statErr == nil {
			items = append(items, CourseItem{Source: file, Output: output, SRTPath: srtPath, TextPath: textPath, TargetName: targetName})
			continue
		}
		compressReq := CompressRequest{
			Path:        file,
			Output:      output,
			Resolution:  req.Resolution,
			Preset:      req.Preset,
			CRF:         req.CRF,
			FPS:         req.FPS,
			FastSkip:    true,
			ExternalSRT: srtPath,
			TargetName:  targetName,
		}
		if req.FastSkip {
			compressReq.FastSkip = true
		}
		if compressReq.Preset == "" {
			compressReq.Preset = "slideshow"
		}
		if compressReq.FPS == "" {
			compressReq.FPS = "1/2"
		}
		if compressReq.Resolution == 0 {
			compressReq.Resolution = -1
		}
		output, out, compressErr := s.compress(ctx, compressReq)
		if compressErr != nil {
			return CourseResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + compressErr.Error())
		}
		items = append(items, CourseItem{Source: file, Output: output, SRTPath: srtPath, TextPath: textPath, TargetName: targetName})
	}

	parts, err := chunkCourseItems(ctx, s, items, req.MaxMergeHours)
	if err != nil {
		return CourseResponse{}, err
	}
	folderName := filepath.Base(targetDir)
	response := CourseResponse{CourseDir: courseDir, Compressed: items, Transcribed: transcribed, Skipped: skipped}
	multipart := len(parts) > 1
	for i, part := range parts {
		suffix := ""
		if multipart {
			suffix = fmt.Sprintf("_Part%d", i+1)
		}
		videoPath := filepath.Join(courseDir, folderName+suffix+"_Slideshow.mp4")
		tmpVideoPath := filepath.Join(courseDir, folderName+suffix+"_tmp.mp4")
		srtPath := filepath.Join(courseDir, folderName+suffix+".srt")
		textPath := filepath.Join(courseDir, folderName+suffix+".txt")
		if err := s.mergeSRTs(ctx, part, srtPath); err != nil {
			return CourseResponse{}, err
		}
		if err := s.mergeVideos(ctx, part, tmpVideoPath); err != nil {
			return CourseResponse{}, err
		}
		if _, err := os.Stat(srtPath); err == nil {
			if err := s.embedSRT(ctx, tmpVideoPath, srtPath, videoPath); err != nil {
				return CourseResponse{}, err
			}
		} else if err := os.Rename(tmpVideoPath, videoPath); err != nil {
			return CourseResponse{}, err
		}
		if err := mergeTranscripts(part, textPath); err != nil {
			return CourseResponse{}, err
		}
		if i == 0 {
			response.VideoPath = videoPath
			response.SRTPath = srtPath
			response.TextPath = textPath
		}
	}
	return response, nil
}

func (s *Service) BackupMetadata(ctx context.Context, req MetadataRequest) (MetadataResponse, error) {
	if strings.TrimSpace(req.Path) == "" {
		return MetadataResponse{}, errors.New("path is required")
	}
	sidecar := req.Sidecar
	if sidecar == "" {
		sidecar = req.Path + ".ffmetadata"
	}
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFmpeg,
		"-y",
		"-i", req.Path,
		"-f", "ffmetadata",
		sidecar,
	)
	if err != nil {
		return MetadataResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return MetadataResponse{Sidecar: sidecar}, nil
}

func (s *Service) RestoreMetadata(ctx context.Context, req MetadataRequest) (MetadataResponse, error) {
	if strings.TrimSpace(req.Path) == "" || strings.TrimSpace(req.Sidecar) == "" {
		return MetadataResponse{}, errors.New("path and sidecar are required")
	}
	output := req.Output
	if output == "" {
		output = strings.TrimSuffix(req.Path, filepathExt(req.Path)) + ".restored" + filepathExt(req.Path)
	}
	out, err := s.runner.CombinedOutput(
		ctx,
		s.tools.FFmpeg,
		"-y",
		"-i", req.Path,
		"-i", req.Sidecar,
		"-map_metadata", "1",
		"-codec", "copy",
		output,
	)
	if err != nil {
		return MetadataResponse{}, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return MetadataResponse{Sidecar: req.Sidecar, Output: output}, nil
}

func (s *Service) Generate(ctx context.Context, req LLMRequest) (LLMResponse, error) {
	if s.provider == nil {
		return LLMResponse{}, errors.New("provider is required")
	}
	if strings.TrimSpace(req.Transcript) == "" {
		return LLMResponse{}, errors.New("transcript is required")
	}
	prompt, err := videoPrompt(req)
	if err != nil {
		return LLMResponse{}, err
	}
	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: req.Model,
		Messages: []provider.Message{
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
		MaxTokens:   3500,
	})
	if err != nil {
		return LLMResponse{}, err
	}
	return LLMResponse{Text: strings.TrimSpace(res.Content)}, nil
}

func summarize(raw []byte) string {
	var payload struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Size     string `json:"size"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	parts := []string{}
	for _, stream := range payload.Streams {
		if stream.CodecType == "video" {
			parts = append(parts, "video="+stream.CodecName)
		}
		if stream.CodecType == "audio" {
			parts = append(parts, "audio="+stream.CodecName)
		}
	}
	if payload.Format.Duration != "" {
		parts = append(parts, "duration="+payload.Format.Duration)
	}
	return strings.Join(parts, " ")
}

func videoPrompt(req LLMRequest) (string, error) {
	title := req.Title
	if title == "" {
		title = "Untitled video"
	}
	switch req.Mode {
	case "", "notes":
		return "Create high-signal study notes for this video transcript. Include headings, bullets, key terms, and action items.\nTitle: " + title + "\n\n" + req.Transcript, nil
	case "tags":
		return "Generate concise searchable tags for this video. Output JSON with keys title, summary, tags, difficulty, topics.\nTitle: " + title + "\n\n" + req.Transcript, nil
	case "course":
		return "Turn this video transcript into a course module plan. Include module title, learning objectives, lesson outline, quiz questions, and prerequisites.\nTitle: " + title + "\n\n" + req.Transcript, nil
	default:
		return "", errors.New("unsupported video LLM mode")
	}
}

type compressPreset struct {
	videoBitrate  string
	audioBitrate  string
	audioChannels int
	nvencPreset   string
	fps           string
}

var compressPresets = map[string]compressPreset{
	"ultralight": {videoBitrate: "150k", audioBitrate: "32k", audioChannels: 1, nvencPreset: "p1", fps: "10"},
	"light":      {videoBitrate: "250k", audioBitrate: "48k", audioChannels: 1, nvencPreset: "p1", fps: "15"},
	"balanced":   {videoBitrate: "400k", audioBitrate: "64k", audioChannels: 1, nvencPreset: "p1", fps: "24"},
	"slideshow":  {videoBitrate: "500k", audioBitrate: "48k", audioChannels: 1, nvencPreset: "p4", fps: "1/20"},
}

func (s *Service) compress(ctx context.Context, req CompressRequest) (string, []byte, error) {
	presetName := req.Preset
	if presetName == "" {
		presetName = "light"
	}
	preset, ok := compressPresets[presetName]
	if !ok {
		return "", nil, fmt.Errorf("unknown preset %q", presetName)
	}
	resolution := req.Resolution
	if presetName == "slideshow" && resolution == 240 {
		resolution = 0
	}
	if resolution == 0 {
		resolution = 240
	}
	if resolution < 0 {
		resolution = 0
	}
	fps := req.FPS
	if fps == "" {
		fps = preset.fps
	}
	output := req.Output
	if output == "" {
		if req.Overwrite {
			output = strings.TrimSuffix(req.Path, filepath.Ext(req.Path)) + ".tmp_compress.mp4"
		} else {
			stem := req.TargetName
			if stem == "" {
				stem = strings.TrimSuffix(filepath.Base(req.Path), filepath.Ext(req.Path))
			}
			suffix := "_slideshow"
			if resolution > 0 {
				suffix = fmt.Sprintf("_%dp", resolution)
			}
			output = filepath.Join(filepath.Dir(req.Path), stem+suffix+".mp4")
		}
	}
	args := []string{"-y", "-v", "error", "-stats"}
	if resolution > 0 {
		args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
		if req.FastSkip {
			args = append(args, "-skip_frame", "nokey")
		}
		args = append(args, "-i", req.Path)
		if req.ExternalSRT != "" && fileExists(req.ExternalSRT) {
			args = append(args, "-i", req.ExternalSRT)
		}
		args = append(args, "-vf", fmt.Sprintf("scale_cuda=-2:%d", resolution))
	} else {
		if req.FastSkip {
			args = append(args, "-hwaccel", "cuda", "-skip_frame", "nokey")
		}
		args = append(args, "-i", req.Path)
		if req.ExternalSRT != "" && fileExists(req.ExternalSRT) {
			args = append(args, "-i", req.ExternalSRT)
		}
	}
	args = append(args, "-c:v", "h264_nvenc", "-preset", preset.nvencPreset, "-tune", "ll", "-r", fps)
	if req.CRF > 0 {
		args = append(args, "-cq", strconv.Itoa(req.CRF), "-b:v", "0")
	} else {
		args = append(args, "-b:v", preset.videoBitrate)
	}
	args = append(args, "-c:a", "aac", "-b:a", preset.audioBitrate, "-ac", strconv.Itoa(preset.audioChannels), "-ar", "22050")
	args = append(args, "-map", "0:v:0", "-map", "0:a:0?")
	if req.ExternalSRT != "" && fileExists(req.ExternalSRT) {
		args = append(args, "-map", "1:s?", "-c:s", "mov_text")
	} else {
		args = append(args, "-map", "0:s?", "-c:s", "mov_text")
	}
	args = append(args, "-map_metadata", "0", "-map_chapters", "0", "-movflags", "+faststart", output)
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFmpeg, args...)
	if err != nil {
		_ = os.Remove(output)
		return output, out, err
	}
	if req.Overwrite {
		finalPath := strings.TrimSuffix(req.Path, filepath.Ext(req.Path)) + ".mp4"
		_ = os.Remove(req.Path)
		if err := os.Rename(output, finalPath); err != nil {
			return output, out, err
		}
		output = finalPath
	}
	return output, out, nil
}

func rawVideoFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if isVideoFile(path) {
			return []string{path}, nil
		}
		return nil, nil
	}
	files := []string{}
	err = filepath.WalkDir(path, func(p string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := strings.ToLower(entry.Name())
			if name == ".aicli_cache" || name == "course" || name == "trash" {
				return filepath.SkipDir
			}
			return nil
		}
		if isVideoFile(p) && !strings.Contains(strings.ToLower(filepath.Base(p)), "slideshow") && !strings.Contains(strings.ToLower(filepath.Base(p)), "merged") {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func isVideoFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v":
		return true
	default:
		return false
	}
}

func (s *Service) prepareTranscriptFiles(ctx context.Context, videoPath, cacheDir, whisperModel string) (string, string, bool, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", false, err
	}
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	cacheSRT := filepath.Join(cacheDir, stem+".srt")
	cacheText := filepath.Join(cacheDir, stem+".txt")
	sidecarSRT := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".srt"
	didTranscribe := false
	if !fileExists(cacheSRT) && fileExists(sidecarSRT) {
		if err := copyFile(sidecarSRT, cacheSRT); err != nil {
			return "", "", false, err
		}
	}
	if !fileExists(cacheSRT) {
		if err := s.transcribeVideo(ctx, videoPath, strings.TrimSuffix(cacheSRT, filepath.Ext(cacheSRT)), whisperModel); err != nil {
			return "", "", false, err
		}
		didTranscribe = true
	}
	if fileExists(cacheSRT) {
		text, err := srtToText(cacheSRT)
		if err != nil {
			return cacheSRT, "", didTranscribe, err
		}
		if err := os.WriteFile(cacheText, []byte(text), 0o644); err != nil {
			return cacheSRT, "", didTranscribe, err
		}
	}
	if !fileExists(cacheSRT) {
		cacheSRT = ""
	}
	if !fileExists(cacheText) {
		cacheText = ""
	}
	if cacheSRT == "" || cacheText == "" {
		return cacheSRT, cacheText, didTranscribe, errors.New("transcription did not produce both .srt and .txt")
	}
	return cacheSRT, cacheText, didTranscribe, nil
}

func (s *Service) transcribeVideo(ctx context.Context, videoPath, outputBase, whisperModel string) error {
	if strings.TrimSpace(s.tools.WhisperCLI) == "" {
		return errors.New("whisper-cli is not configured")
	}
	if whisperModel == "" {
		whisperModel = "large-v3"
	}
	args := []string{"-m", whisperModel, "-f", videoPath, "-osrt", "-otxt", "-of", outputBase}
	out, err := s.runner.CombinedOutput(ctx, s.tools.WhisperCLI, args...)
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return nil
}

func courseTargetName(videoPath string, used map[string]int) string {
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	sanitized := sanitizeCourseName(stem)
	if sanitized == "" {
		sanitized = "video"
	}
	suffix := stem
	if len(suffix) > 15 {
		suffix = suffix[len(suffix)-15:]
	}
	target := sanitizeCourseName(sanitized + "_" + suffix)
	if count := used[target]; count > 0 {
		used[target] = count + 1
		return fmt.Sprintf("%s_%d", target, count+1)
	}
	used[target] = 1
	return target
}

func sanitizeCourseName(value string) string {
	re := regexp.MustCompile(`[^A-Za-z0-9 ._-]+`)
	return strings.TrimSpace(re.ReplaceAllString(value, ""))
}

func chunkCourseItems(ctx context.Context, svc *Service, items []CourseItem, maxMergeHours float64) ([][]CourseItem, error) {
	if len(items) == 0 {
		return nil, errors.New("no compressed videos to merge")
	}
	if maxMergeHours <= 0 {
		return [][]CourseItem{items}, nil
	}
	limit := maxMergeHours * 3600
	chunks := [][]CourseItem{}
	current := []CourseItem{}
	currentSeconds := 0.0
	for _, item := range items {
		duration, err := svc.duration(ctx, item.Source)
		if err != nil {
			duration = 0
		}
		if len(current) > 0 && currentSeconds+duration > limit {
			chunks = append(chunks, current)
			current = nil
			currentSeconds = 0
		}
		current = append(current, item)
		currentSeconds += duration
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks, nil
}

func (s *Service) duration(ctx context.Context, videoPath string) (float64, error) {
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFprobe, "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", videoPath)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func (s *Service) mergeVideos(ctx context.Context, items []CourseItem, outputPath string) error {
	if len(items) == 0 {
		return errors.New("no videos to merge")
	}
	listPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".txt"
	var builder strings.Builder
	for _, item := range items {
		escaped := strings.ReplaceAll(item.Output, "'", "'\\''")
		builder.WriteString("file '")
		builder.WriteString(escaped)
		builder.WriteString("'\n")
	}
	if err := os.WriteFile(listPath, []byte(builder.String()), 0o644); err != nil {
		return err
	}
	defer os.Remove(listPath)
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFmpeg, "-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", outputPath)
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	return nil
}

func (s *Service) mergeSRTs(ctx context.Context, items []CourseItem, outputPath string) error {
	var builder strings.Builder
	offset := time.Duration(0)
	index := 1
	for _, item := range items {
		if item.SRTPath != "" && fileExists(item.SRTPath) {
			blocks, err := parseSRT(item.SRTPath)
			if err != nil {
				return err
			}
			for _, block := range blocks {
				builder.WriteString(strconv.Itoa(index))
				builder.WriteString("\n")
				builder.WriteString(formatSRTTime(block.start + offset))
				builder.WriteString(" --> ")
				builder.WriteString(formatSRTTime(block.end + offset))
				builder.WriteString("\n")
				builder.WriteString(block.text)
				builder.WriteString("\n\n")
				index++
			}
		}
		duration, err := s.duration(ctx, item.Output)
		if err != nil {
			duration = 0
		}
		offset += time.Duration(duration * float64(time.Second))
	}
	if builder.Len() == 0 {
		return nil
	}
	return os.WriteFile(outputPath, []byte(builder.String()), 0o644)
}

func (s *Service) embedSRT(ctx context.Context, videoPath, srtPath, outputPath string) error {
	out, err := s.runner.CombinedOutput(ctx, s.tools.FFmpeg, "-y", "-v", "quiet", "-i", videoPath, "-i", srtPath, "-map", "0:v:0", "-map", "0:a?", "-map", "1:s:0", "-c", "copy", "-c:s", "mov_text", outputPath)
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	_ = os.Remove(videoPath)
	return nil
}

func mergeTranscripts(items []CourseItem, outputPath string) error {
	var builder strings.Builder
	for _, item := range items {
		if item.TextPath == "" || !fileExists(item.TextPath) {
			continue
		}
		data, err := os.ReadFile(item.TextPath)
		if err != nil {
			return err
		}
		stem := strings.TrimSuffix(filepath.Base(item.TextPath), filepath.Ext(item.TextPath))
		builder.WriteString("--- Segment: ")
		builder.WriteString(stem)
		builder.WriteString(" ---\n\n")
		builder.Write(data)
		builder.WriteString("\n\n")
	}
	return os.WriteFile(outputPath, []byte(builder.String()), 0o644)
}

type srtBlock struct {
	start time.Duration
	end   time.Duration
	text  string
}

func parseSRT(path string) ([]srtBlock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	chunks := regexp.MustCompile(`\r?\n\r?\n`).Split(strings.TrimSpace(string(data)), -1)
	blocks := []srtBlock{}
	for _, chunk := range chunks {
		lines := strings.Split(strings.ReplaceAll(chunk, "\r\n", "\n"), "\n")
		if len(lines) < 3 || !strings.Contains(lines[1], " --> ") {
			continue
		}
		parts := strings.SplitN(lines[1], " --> ", 2)
		start, err := parseSRTTime(strings.TrimSpace(parts[0]))
		if err != nil {
			continue
		}
		end, err := parseSRTTime(strings.TrimSpace(parts[1]))
		if err != nil {
			continue
		}
		blocks = append(blocks, srtBlock{start: start, end: end, text: strings.Join(lines[2:], "\n")})
	}
	return blocks, nil
}

func parseSRTTime(value string) (time.Duration, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 {
		return 0, errors.New("invalid srt timestamp")
	}
	secParts := strings.Split(parts[2], ",")
	if len(secParts) != 2 {
		return 0, errors.New("invalid srt timestamp")
	}
	hours, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	mins, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	secs, err := strconv.Atoi(secParts[0])
	if err != nil {
		return 0, err
	}
	millis, err := strconv.Atoi(secParts[1])
	if err != nil {
		return 0, err
	}
	return time.Duration(hours)*time.Hour + time.Duration(mins)*time.Minute + time.Duration(secs)*time.Second + time.Duration(millis)*time.Millisecond, nil
}

func formatSRTTime(value time.Duration) string {
	if value < 0 {
		value = 0
	}
	totalMillis := value.Milliseconds()
	millis := totalMillis % 1000
	totalSeconds := totalMillis / 1000
	seconds := totalSeconds % 60
	totalMinutes := totalSeconds / 60
	minutes := totalMinutes % 60
	hours := totalMinutes / 60
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func srtToText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := []string{}
	timestampLine := regexp.MustCompile(`\d{2}:\d{2}:\d{2},\d{3}\s*-->\s*\d{2}:\d{2}:\d{2},\d{3}`)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, err := strconv.Atoi(line); err == nil {
			continue
		}
		if timestampLine.MatchString(line) {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, " "), nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func filepathExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			break
		}
	}
	return ""
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
