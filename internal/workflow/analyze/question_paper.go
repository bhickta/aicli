package analyze

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type PrintedQuestion struct {
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
}

func attachPrintedQuestionPrompts(questions []Question, printed []PrintedQuestion) []Question {
	prompts := make(map[string]string, len(printed))
	for _, item := range printed {
		key := questionReference(item.Label)
		prompt := strings.TrimSpace(item.Prompt)
		if key == "" || prompt == "" || prompts[key] != "" {
			continue
		}
		prompts[key] = prompt
	}
	for index := range questions {
		if prompt := prompts[questionReference(questions[index].Label)]; prompt != "" {
			questions[index].Title = prompt
		}
	}
	return questions
}

func questionReference(label string) string {
	var normalized strings.Builder
	for _, char := range strings.ToLower(strings.TrimSpace(label)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			normalized.WriteRune(char)
		}
	}
	value := strings.TrimPrefix(normalized.String(), "question")
	value = strings.TrimPrefix(value, "q")
	if value == "" {
		return ""
	}
	first, _ := utf8.DecodeRuneInString(value)
	if !unicode.IsDigit(first) {
		return ""
	}
	return value
}

func countQuestionsWithTitles(questions []Question) int {
	count := 0
	for _, question := range questions {
		if strings.TrimSpace(question.Title) != "" {
			count++
		}
	}
	return count
}
