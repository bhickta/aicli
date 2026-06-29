package analyze

import "strings"

func answerBearingPages(pages []Page) []Page {
	out := make([]Page, 0, len(pages))
	for _, page := range pages {
		if isOCRFailureText(page.Text) || isCoverOrIndexPage(page.Text) {
			continue
		}
		out = append(out, page)
	}
	return out
}

func isCoverOrIndexPage(text string) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, "index table") && strings.Contains(lower, "name of candidate") {
		return true
	}
	if strings.Contains(lower, "test code") && strings.Contains(lower, "maximum marks") && strings.Contains(lower, "instructions") {
		return true
	}
	return false
}

func questionsForPages(questions []Question, pages []Page) []Question {
	allowed := map[int]bool{}
	for _, page := range pages {
		allowed[page.Number] = true
	}
	out := make([]Question, 0, len(questions))
	for _, question := range questions {
		if questionHasAllowedPage(question, allowed) {
			out = append(out, question)
		}
	}
	return out
}

func questionHasAllowedPage(question Question, allowed map[int]bool) bool {
	for _, page := range question.SourcePages {
		if allowed[page] {
			return true
		}
	}
	return len(question.SourcePages) == 0 && len(allowed) > 0
}
