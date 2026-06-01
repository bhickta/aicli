package training

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
)

func findInboxTrainingFiles(dataRoot string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dataRoot, "inbox-runs", "*", "training", "zettel-inbox-chat.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("find inbox training archives: %w", err)
	}
	if files == nil {
		files = []string{}
	}
	sort.Strings(files)
	return files, nil
}

func cleanExchange(exchange archivepkg.LLMExchange) (chatExample, string) {
	switch {
	case strings.TrimSpace(exchange.Error) != "":
		return chatExample{}, "failed-exchange"
	case exchange.Workflow != exportWorkflow:
		return chatExample{}, "not-target-workflow"
	case exchange.Step != exportStep:
		return chatExample{}, "not-target-step"
	case exchange.ParsedFormat != exportParsedFormat:
		return chatExample{}, "not-target-format"
	}

	messages := normalizeRequestMessages(exchange.Request.Messages)
	if len(messages) == 0 {
		return chatExample{}, "empty-messages"
	}
	assistant := strings.TrimSpace(normalizeContent(exchange.Response.Content))
	if assistant == "" {
		return chatExample{}, "empty-response"
	}
	messages = append(messages, provider.Message{
		Role:    "assistant",
		Content: assistant,
	})
	return chatExample{Messages: messages}, ""
}

func normalizeRequestMessages(messages []provider.Message) []provider.Message {
	out := make([]provider.Message, 0, len(messages))
	for _, message := range messages {
		role := strings.TrimSpace(strings.ToLower(message.Role))
		content := strings.TrimSpace(normalizeContent(message.Content))
		if role == "" || content == "" {
			continue
		}
		out = append(out, provider.Message{
			Role:    role,
			Content: content,
		})
	}
	return out
}

func normalizeContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return strings.TrimSpace(content)
}

func splitRecords(records []exportRecord) ([]exportRecord, []exportRecord) {
	evalCount := len(records) / 20
	if evalCount >= len(records) {
		evalCount = 0
	}
	evalRecords := records[:evalCount]
	trainRecords := records[evalCount:]
	return trainRecords, evalRecords
}
