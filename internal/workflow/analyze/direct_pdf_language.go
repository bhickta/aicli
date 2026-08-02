package analyze

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

func validateDirectPDFQuestionAnalysisLanguage(questions []Question) error {
	for _, question := range questions {
		if question.Dimensions == nil {
			continue
		}
		encoded, err := json.Marshal(question.Dimensions)
		if err != nil {
			return fmt.Errorf("encode direct PDF analysis for question %q: %w", question.ID, err)
		}
		if phrase := directPDFInternalProcessingPhrase(string(encoded)); phrase != "" {
			return newIncompleteDirectPDFError(
				"direct PDF question %q attributes visible answer quality to internal processing (%s)",
				firstNonBlank(question.Label, question.ID),
				phrase,
			)
		}
	}
	return nil
}

func validateDirectPDFVisibleAnalysis(questions []Question, report string, warnings []string) error {
	if err := validateDirectPDFQuestionAnalysisLanguage(questions); err != nil {
		return err
	}
	for _, item := range append([]string{report}, warnings...) {
		if phrase := directPDFInternalProcessingPhrase(item); phrase != "" {
			return fmt.Errorf("direct PDF visible analysis refers to internal processing (%s)", phrase)
		}
	}
	return nil
}

func directPDFInternalProcessingPhrase(value string) string {
	tokens := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	for index, token := range tokens {
		if token == "chunk" || token == "chunks" {
			return token
		}
		if index+1 >= len(tokens) {
			continue
		}
		next := tokens[index+1]
		switch token {
		case "extraction":
			if next == "boundary" || next == "boundaries" || next == "window" || next == "windows" {
				return token + " " + next
			}
		case "processing":
			if next == "boundary" || next == "boundaries" || next == "detail" || next == "details" {
				return token + " " + next
			}
		case "technical":
			if next == "processing" || next == "loss" {
				return token + " " + next
			}
		case "overlap":
			if next == "window" || next == "windows" {
				return token + " " + next
			}
		}
	}
	return ""
}
