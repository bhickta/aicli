package lecture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/provider"
)

func (s *Service) Run(ctx context.Context, req Request, progress ProgressFunc) (Response, error) {
	if s.provider == nil {
		return Response{}, errors.New("provider is required")
	}
	if s.runner == nil {
		return Response{}, errors.New("tool runner is required")
	}
	total := 3
	if req.SynthesizeAudio {
		total = 4
	}
	progressUnits(progress, "collecting Obsidian notes", 0, total)
	notes, skipped, inputChars, err := collectNotes(req)
	if err != nil {
		return Response{}, err
	}
	progressUnits(progress, "generating crisp lecture script", 1, total)
	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: req.Model,
		Messages: []provider.Message{
			{Role: "user", Content: lecturePrompt(notes, req.Style)},
		},
		Temperature: 0.2,
		MaxTokens:   7000,
	})
	if err != nil {
		return Response{}, err
	}
	script := strings.TrimSpace(res.Content)
	if script == "" {
		return Response{}, errors.New("lecture model returned empty script")
	}

	progressUnits(progress, "writing lecture artifacts", 2, total)
	id := fmt.Sprintf("lecture-%d", time.Now().UTC().UnixNano())
	title := outputTitle(req, notes)
	artifactRoot := s.artifactDir
	if artifactRoot == "" {
		artifactRoot = filepath.Join(os.TempDir(), "aicli-lectures")
	}
	outDir := filepath.Join(artifactRoot, id)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Response{}, err
	}
	scriptPath := filepath.Join(outDir, safeFileName(title)+".md")
	if err := os.WriteFile(scriptPath, []byte(script+"\n"), 0o644); err != nil {
		return Response{}, err
	}
	sourceNotes := make([]string, 0, len(notes))
	for _, note := range notes {
		sourceNotes = append(sourceNotes, note.Rel)
	}
	if err := writeManifest(filepath.Join(outDir, "manifest.json"), req, sourceNotes, skipped, inputChars); err != nil {
		return Response{}, err
	}

	response := Response{
		Kind:         "lecture",
		ID:           id,
		Title:        title,
		Script:       script,
		ScriptPath:   scriptPath,
		ScriptURL:    artifactURL(artifactRoot, scriptPath),
		SourceNotes:  sourceNotes,
		SkippedNotes: skipped,
		InputChars:   inputChars,
	}
	if req.SynthesizeAudio {
		progressUnits(progress, "synthesizing lecture audio with SOAR TTS", 3, total)
		audioPath := filepath.Join(outDir, safeFileName(title)+".wav")
		command, args, err := s.ttsCommand(req, scriptPath, audioPath)
		if err != nil {
			return Response{}, err
		}
		out, err := s.runner.CombinedOutput(ctx, command, args...)
		if err != nil {
			return Response{}, fmt.Errorf("tts failed: %s: %w", strings.TrimSpace(string(out)), err)
		}
		response.AudioPath = audioPath
		response.AudioURL = artifactURL(artifactRoot, audioPath)
		response.TTSCommandLine = append([]string{command}, args...)
	}
	progressUnits(progress, "lecture ready", total, total)
	return response, nil
}

func progressUnits(progress ProgressFunc, stage string, completed int, total int) {
	if progress != nil {
		progress(stage, completed, total, "stage")
	}
}

func outputTitle(req Request, notes []noteInput) string {
	if strings.TrimSpace(req.OutputName) != "" {
		return strings.TrimSpace(req.OutputName)
	}
	if len(notes) == 1 {
		name := strings.TrimSuffix(filepath.Base(notes[0].Rel), filepath.Ext(notes[0].Rel))
		if name != "" {
			return name + " Lecture"
		}
	}
	return "UPSC Notes Lecture"
}

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "lecture"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "", "\"", "", "<", "", ">", "", "|", "-")
	name = replacer.Replace(name)
	name = strings.Join(strings.Fields(name), " ")
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func writeManifest(path string, req Request, sourceNotes []string, skipped int, inputChars int) error {
	data, err := json.MarshalIndent(map[string]any{
		"vault_path":    req.VaultPath,
		"source_path":   req.SourcePath,
		"source_notes":  sourceNotes,
		"skipped_notes": skipped,
		"input_chars":   inputChars,
		"style":         req.Style,
		"created_at":    time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func artifactURL(root string, path string) string {
	if root == "" || path == "" {
		return ""
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return "/artifacts/lectures/" + strings.Join(parts, "/")
}
