package analyze

func answerBearingPages(pages []Page) []Page {
	out := make([]Page, 0, len(pages))
	for _, page := range pages {
		if isOCRFailureText(page.Text) {
			continue
		}
		switch normalizePageKind(page.Kind) {
		case "answer", "unknown":
			out = append(out, page)
		}
	}
	return out
}

func pagesForQuestionSplit(pages []Page) []Page {
	out := make([]Page, 0, len(pages))
	for _, page := range pages {
		if isOCRFailureText(page.Text) {
			continue
		}
		switch normalizePageKind(page.Kind) {
		case "answer", "question_paper", "unknown":
			out = append(out, page)
		}
	}
	return out
}

func applyPageClassifications(pages []Page, classifications []PageClassification) []Page {
	byPage := make(map[int]PageClassification, len(classifications))
	for _, classification := range classifications {
		byPage[classification.PageNumber] = classification
	}
	for index := range pages {
		classification, ok := byPage[pages[index].Number]
		if !ok {
			continue
		}
		pages[index].Kind = classification.Kind
		pages[index].KindConfidence = classification.Confidence
		pages[index].ClassificationReason = classification.Reason
		pages[index].OCRConfidence = classification.OCRConfidence
		pages[index].OCRIssues = append([]string(nil), classification.OCRIssues...)
	}
	return pages
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
