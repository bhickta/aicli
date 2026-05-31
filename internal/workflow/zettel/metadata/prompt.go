package metadata

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func metadataMessages(path string, content string) []provider.Message {
	return []provider.Message{
		{
			Role: "system",
			Content: strings.Join([]string{
				"You generate high-quality metadata for UPSC zettelkasten notes.",
				"Return JSON only. Do not return markdown fences, prose, comments, or extra keys.",
				"JSON schema:",
				`{"title":"...","summary_keywords":"...","recall_questions":["...","...","..."]}`,
				"title: detailed, specific, filename-quality title for the whole note.",
				"summary_keywords: one short keyword-only line; cover every important concept, actor, place, scheme, date, number, example, and causal relation from the note; avoid sentences.",
				"recall_questions: 3 to 5 broad questions; together they must force recall of the whole note.",
				"Do not add external knowledge.",
				"Do not omit numbers, names, places, examples, lists, definitions, or causal chains from the metadata.",
				"Do not use markdown formatting in field values.",
			}, "\n"),
		},
		{
			Role: "user",
			Content: strings.Join([]string{
				"NOTE PATH:",
				path,
				"",
				"NOTE CONTENT:",
				content,
			}, "\n"),
		},
	}
}

func parseGeneratedMetadata(text string) (generatedMetadata, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return generatedMetadata{}, errors.New("metadata response was empty")
	}
	text = stripJSONEnvelope(text)
	var item generatedMetadata
	if err := json.Unmarshal([]byte(text), &item); err != nil {
		return generatedMetadata{}, err
	}
	item = normalizeGeneratedMetadata(item)
	if item.Title == "" {
		return generatedMetadata{}, errors.New("metadata title was empty")
	}
	if item.SummaryKeywords == "" {
		return generatedMetadata{}, errors.New("metadata summary_keywords was empty")
	}
	if len(item.RecallQuestions) < 3 {
		return generatedMetadata{}, errors.New("metadata recall_questions must contain at least 3 questions")
	}
	return item, nil
}

func stripJSONEnvelope(text string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		lines := strings.Split(text, "\n")
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
			text = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return text
}

func normalizeGeneratedMetadata(item generatedMetadata) generatedMetadata {
	item.Title = cleanMetadataValue(item.Title)
	item.SummaryKeywords = cleanMetadataValue(item.SummaryKeywords)
	questions := make([]string, 0, len(item.RecallQuestions))
	for _, question := range item.RecallQuestions {
		question = cleanMetadataValue(question)
		if question == "" {
			continue
		}
		questions = append(questions, question)
		if len(questions) >= 5 {
			break
		}
	}
	item.RecallQuestions = questions
	return item
}

func cleanMetadataValue(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`")
	return strings.Join(strings.Fields(value), " ")
}
