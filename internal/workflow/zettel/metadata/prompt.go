package metadata

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/bhickta/aicli/internal/provider"
)

func metadataMessages(_ string, content string) []provider.Message {
	return []provider.Message{
		{
			Role: "system",
			Content: strings.Join([]string{
				"You generate high-quality metadata for UPSC zettelkasten notes.",
				"Return JSON only. Do not return markdown fences, prose, comments, or extra keys.",
				"JSON schema:",
				`{"title":"...","summary_keywords":"...","recall_questions":["..."]}`,
				"title: detailed, specific, filename-quality title for the actual body content.",
				"summary_keywords: compact keyword-only line; max 280 characters; include the most important concepts, actors, places, dates, numbers, examples, and causal relations; no sentences.",
				"recall_questions: 1 to 3 telegraphic recall prompts, not grammatical questions.",
				"Recall prompt style: topic terms only, use +, /, ->, commas; no question marks.",
				"Good recall prompt: Chalcolithic art/culture -> jewelry + beads + village evolution.",
				"Bad recall prompt: What characteristics define the note's description of art and culture?",
				"Do not mention note, text, source, content, description, according to, status, course, series, or part number.",
				"Do not add external knowledge.",
				"Do not omit numbers, names, places, examples, lists, definitions, or causal chains from the metadata.",
				"The user message contains body content only. Ignore any temptation to infer from filename, path, frontmatter, or course metadata.",
				"Do not use markdown formatting in field values.",
			}, "\n"),
		},
		{
			Role: "user",
			Content: strings.Join([]string{
				"BODY CONTENT:",
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
	if len(item.RecallQuestions) == 0 {
		return generatedMetadata{}, errors.New("metadata recall_questions must contain at least 1 recall prompt")
	}
	if err := validateGeneratedMetadata(item); err != nil {
		return generatedMetadata{}, err
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
		if len(questions) >= 3 {
			break
		}
	}
	item.RecallQuestions = questions
	return item
}

func validateGeneratedMetadata(item generatedMetadata) error {
	for _, value := range []struct {
		field string
		text  string
	}{
		{field: "title", text: item.Title},
		{field: "summary_keywords", text: item.SummaryKeywords},
	} {
		if containsMetadataLeak(value.text) {
			return fmt.Errorf("metadata %s references source/status/note metadata", value.field)
		}
	}
	if len([]rune(item.SummaryKeywords)) > 320 {
		return errors.New("metadata summary_keywords is too long")
	}
	for _, prompt := range item.RecallQuestions {
		if err := validateRecallPrompt(prompt); err != nil {
			return err
		}
	}
	return nil
}

func validateRecallPrompt(prompt string) error {
	if strings.Contains(prompt, "?") {
		return fmt.Errorf("recall prompt must not be a grammatical question: %q", prompt)
	}
	if startsWithQuestionWord(prompt) {
		return fmt.Errorf("recall prompt starts with a question word: %q", prompt)
	}
	if containsMetadataLeak(prompt) || containsRecallMetaWording(prompt) {
		return fmt.Errorf("recall prompt references note/source metadata: %q", prompt)
	}
	return nil
}

func startsWithQuestionWord(value string) bool {
	first := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	if len(first) == 0 {
		return false
	}
	switch strings.Trim(first[0], "\"'`.,:;") {
	case "what", "how", "why", "when", "where", "which", "who", "whom":
		return true
	default:
		return false
	}
}

func containsMetadataLeak(value string) bool {
	text := strings.ToLower(value)
	blocked := []string{
		"source:",
		"source metadata",
		"note status",
		"status:",
		"study iq",
		"rs sharma",
		"history optional",
		"part number",
		"part ",
		"course",
		"series",
	}
	for _, term := range blocked {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func containsRecallMetaWording(value string) bool {
	words := wordSet(value)
	blocked := []string{
		"note",
		"text",
		"content",
		"description",
		"described",
		"discussed",
		"according",
		"source",
		"status",
	}
	for _, word := range blocked {
		if words[word] {
			return true
		}
	}
	return false
}

func wordSet(value string) map[string]bool {
	words := map[string]bool{}
	for _, field := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if field != "" {
			words[field] = true
		}
	}
	return words
}

func cleanMetadataValue(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`")
	return strings.Join(strings.Fields(value), " ")
}
