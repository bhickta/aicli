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

func validExchange(index int) archivepkg.LLMExchange {
	return validExchangeWith(
		index,
		exportWorkflow,
		exportStep,
		exportParsedFormat,
		"",
		fmt.Sprintf("final merged notes %02d", index),
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
				{Role: "user", Content: fmt.Sprintf("source note %02d", index)},
			},
		},
		Response: provider.ChatResponse{Content: response},
	}
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

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
