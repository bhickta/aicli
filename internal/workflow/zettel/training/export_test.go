package training

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bhickta/aicli/internal/provider"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
)

func TestRunnerExportFiltersDedupesAndSplitsCleanMergeExamples(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	dataDir := t.TempDir()
	trainingPath := filepath.Join(dataDir, "inbox-runs", "run-1", "training", "zettel-inbox-chat.jsonl")

	exchanges := make([]archivepkg.LLMExchange, 0, 27)
	for i := range 20 {
		exchanges = append(exchanges, validExchange(i))
	}
	exchanges = append(exchanges, exchanges[0])
	exchanges = append(exchanges,
		validExchangeWith(100, "zettel-metadata", exportStep, exportParsedFormat, "", "metadata"),
		validExchangeWith(101, exportWorkflow, "judge-destinations", exportParsedFormat, "", "judge"),
		validExchangeWith(102, exportWorkflow, exportStep, "pending", "", "pending"),
		validExchangeWith(103, exportWorkflow, exportStep, exportParsedFormat, "failed", "failed"),
		validExchangeWith(104, exportWorkflow, exportStep, exportParsedFormat, "", ""),
	)
	writeTrainingJSONL(t, trainingPath, exchanges, "{bad json}\n")

	resp, err := New().Export(context.Background(), TrainingExportRequest{
		Options: Options{
			VaultPath:  vaultDir,
			DataFolder: dataDir,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if resp.ScannedCount != 27 {
		t.Fatalf("scanned count = %d, want 27", resp.ScannedCount)
	}
	if resp.ExportedCount != 20 || resp.TrainCount != 19 || resp.EvalCount != 1 {
		t.Fatalf("counts = exported:%d train:%d eval:%d, want 20/19/1", resp.ExportedCount, resp.TrainCount, resp.EvalCount)
	}
	if resp.DuplicateCount != 1 {
		t.Fatalf("duplicate count = %d, want 1", resp.DuplicateCount)
	}
	wantSkipped := map[string]int{
		"malformed-json":      1,
		"not-target-workflow": 1,
		"not-target-step":     1,
		"not-target-format":   1,
		"failed-exchange":     1,
		"empty-response":      1,
	}
	for reason, count := range wantSkipped {
		if resp.SkippedByReason[reason] != count {
			t.Fatalf("skipped[%s] = %d, want %d; all=%#v", reason, resp.SkippedByReason[reason], count, resp.SkippedByReason)
		}
	}

	assertChatJSONL(t, resp.TrainPath, 19)
	assertChatJSONL(t, resp.EvalPath, 1)
	assertShareGPTJSONL(t, resp.ShareGPTTrainPath, 19)
	assertShareGPTJSONL(t, resp.ShareGPTEvalPath, 1)
	if _, err := os.Stat(resp.ManifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}

	second, err := New().Export(context.Background(), TrainingExportRequest{
		Options: Options{
			VaultPath:  vaultDir,
			DataFolder: dataDir,
		},
	}, nil)
	if err != nil {
		t.Fatalf("second Export() error = %v", err)
	}
	if readFile(t, resp.EvalPath) != readFile(t, second.EvalPath) {
		t.Fatal("eval split changed between identical exports")
	}
}

func TestRunnerExportStrictFiltersTrainingNoise(t *testing.T) {
	t.Parallel()

	vaultDir := t.TempDir()
	dataDir := t.TempDir()
	trainingPath := filepath.Join(dataDir, "inbox-runs", "run-1", "training", "zettel-inbox-chat.jsonl")

	exchanges := make([]archivepkg.LLMExchange, 0, 18)
	for i := range 10 {
		exchanges = append(exchanges, validExchange(i))
	}
	exchanges = append(exchanges,
		validExchangeWith(20, exportWorkflow, exportStep, exportParsedFormat, "", "```markdown\n- wrapped\n```"),
		validExchangeWith(21, exportWorkflow, exportStep, exportParsedFormat, "", strings.Join([]string{
			"BEGIN_NOTE zettelkasten/dest.md",
			"---",
			"Status: Read",
			"---",
			"---",
			"Source: duplicate",
			"---",
			"- **Fact**: duplicate frontmatter.",
			"END_NOTE",
		}, "\n")),
		validExchangeWith(22, exportWorkflow, exportStep, exportParsedFormat, "", "BEGIN_NOTE zettelkasten/dest.md\n- missing end"),
		exchangeWithoutCandidates(23),
		exchangeWithSystemPrompt(24, "old prompt"),
	)
	writeTrainingJSONL(t, trainingPath, exchanges, "")

	resp, err := New().Export(context.Background(), TrainingExportRequest{
		Options: Options{
			VaultPath:  vaultDir,
			DataFolder: dataDir,
		},
		Strict: true,
	}, nil)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	if !resp.Strict {
		t.Fatal("strict flag was not preserved in response")
	}
	if resp.ExportedCount != 10 || resp.TrainCount != 10 || resp.EvalCount != 0 {
		t.Fatalf("counts = exported:%d train:%d eval:%d, want 10/10/0", resp.ExportedCount, resp.TrainCount, resp.EvalCount)
	}
	wantSkipped := map[string]int{
		"assistant-code-fence":        1,
		"duplicate-frontmatter":       1,
		"bad-note-boundaries":         1,
		"missing-semantic-candidates": 1,
		"non-primary-system-prompt":   1,
	}
	for reason, count := range wantSkipped {
		if resp.SkippedByReason[reason] != count {
			t.Fatalf("skipped[%s] = %d, want %d; all=%#v", reason, resp.SkippedByReason[reason], count, resp.SkippedByReason)
		}
	}
	if resp.Quality.ExamplesWithCodeFences != 0 ||
		resp.Quality.ExamplesWithDuplicateFrontmatter != 0 ||
		resp.Quality.ExamplesWithBadNoteBoundaries != 0 ||
		resp.Quality.ExamplesWithoutSemanticCandidates != 0 {
		t.Fatalf("quality report still has strict red flags: %#v", resp.Quality)
	}
	if resp.Quality.SystemPromptVariants != 1 || resp.Quality.TotalFinalNotes != 10 {
		t.Fatalf("quality report = %#v, want one prompt variant and 10 final notes", resp.Quality)
	}
	assertChatJSONL(t, resp.TrainPath, 10)
	assertShareGPTJSONL(t, resp.ShareGPTTrainPath, 10)
}

func validExchange(index int) archivepkg.LLMExchange {
	return validExchangeWith(
		index,
		exportWorkflow,
		exportStep,
		exportParsedFormat,
		"",
		strings.Join([]string{
			fmt.Sprintf("BEGIN_NOTE zettelkasten/dest-%02d.md", index),
			"---",
			"Status: Read",
			"---",
			fmt.Sprintf("- **Final Fact**: merged notes %02d.", index),
			"END_NOTE",
		}, "\n"),
	)
}

func validExchangeWith(
	index int,
	workflow string,
	step string,
	format string,
	exchangeErr string,
	response string,
) archivepkg.LLMExchange {
	return archivepkg.LLMExchange{
		Workflow:     workflow,
		Step:         step,
		SourcePath:   "in/source.md",
		Error:        exchangeErr,
		ParsedFormat: format,
		Request: provider.ChatRequest{
			Messages: []provider.Message{
				{Role: "system", Content: "merge notes"},
				{Role: "user", Content: strings.Join([]string{
					fmt.Sprintf("SOURCE PATH:\nin/source-%02d.md", index),
					fmt.Sprintf("SOURCE NOTE:\n- source note %02d", index),
					"SEMANTIC DESTINATION CANDIDATES:",
					"CANDIDATE 1",
					"PATH: zettelkasten/dest.md",
				}, "\n\n")},
			},
		},
		Response: provider.ChatResponse{Content: response},
	}
}

func exchangeWithoutCandidates(index int) archivepkg.LLMExchange {
	exchange := validExchange(index)
	exchange.Request.Messages[1].Content = fmt.Sprintf("SOURCE PATH:\nin/source-%02d.md\n\nSOURCE NOTE:\n- source note", index)
	return exchange
}

func exchangeWithSystemPrompt(index int, systemPrompt string) archivepkg.LLMExchange {
	exchange := validExchange(index)
	exchange.Request.Messages[0].Content = systemPrompt
	return exchange
}

func writeTrainingJSONL(t *testing.T, path string, exchanges []archivepkg.LLMExchange, trailing string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	var builder strings.Builder
	for _, exchange := range exchanges {
		data, err := json.Marshal(exchange)
		if err != nil {
			t.Fatal(err)
		}
		builder.Write(data)
		builder.WriteByte('\n')
	}
	builder.WriteString(trailing)
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertChatJSONL(t *testing.T, path string, wantCount int) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(readFile(t, path)), "\n")
	if wantCount == 0 {
		if strings.TrimSpace(readFile(t, path)) != "" {
			t.Fatalf("%s was not empty", path)
		}
		return
	}
	if len(lines) != wantCount {
		t.Fatalf("%s line count = %d, want %d", path, len(lines), wantCount)
	}
	for _, line := range lines {
		var example chatExample
		if err := json.Unmarshal([]byte(line), &example); err != nil {
			t.Fatalf("parse exported example: %v\n%s", err, line)
		}
		if len(example.Messages) != 3 {
			t.Fatalf("messages = %#v, want system/user/assistant", example.Messages)
		}
		last := example.Messages[len(example.Messages)-1]
		if last.Role != "assistant" || strings.TrimSpace(last.Content) == "" {
			t.Fatalf("assistant message = %#v, want non-empty assistant final message", last)
		}
	}
}

func assertShareGPTJSONL(t *testing.T, path string, wantCount int) {
	t.Helper()
	content := strings.TrimSpace(readFile(t, path))
	if wantCount == 0 {
		if content != "" {
			t.Fatalf("%s was not empty", path)
		}
		return
	}
	lines := strings.Split(content, "\n")
	if len(lines) != wantCount {
		t.Fatalf("%s line count = %d, want %d", path, len(lines), wantCount)
	}
	for _, line := range lines {
		var example shareGPTExample
		if err := json.Unmarshal([]byte(line), &example); err != nil {
			t.Fatalf("parse sharegpt example: %v\n%s", err, line)
		}
		if len(example.Conversations) != 3 {
			t.Fatalf("conversations = %#v, want 3", example.Conversations)
		}
		if example.Conversations[1].From != "human" || example.Conversations[2].From != "gpt" {
			t.Fatalf("sharegpt roles = %#v", example.Conversations)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
