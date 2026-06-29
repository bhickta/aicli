package analyze

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type oneShotPDFManifest struct {
	Pages     []oneShotPDFPage     `json:"pages"`
	Questions []oneShotPDFQuestion `json:"questions"`
	Report    string               `json:"report"`
}

type oneShotPDFPage struct {
	Number       int    `json:"number"`
	Name         string `json:"name"`
	Text         string `json:"text"`
	UnclearCount int    `json:"unclear_count"`
}

type oneShotPDFQuestion struct {
	ID             string             `json:"id"`
	Label          string             `json:"label"`
	Title          string             `json:"title"`
	SourcePages    []int              `json:"source_pages"`
	AnswerMarkdown string             `json:"answer_markdown"`
	Dimensions     QuestionDimensions `json:"dimensions"`
}

func oneShotPDFPrompt(pdfName string) string {
	return `Analyze this entire UPSC topper answer-copy PDF with Gemini Flash-Lite.
Extract page source notes, every question with full answer text, per-question dimensions, and one final copy-level report.

Return valid JSON only. No markdown fences, no trailing commas, no prose outside JSON.
Escape double quotes inside strings as \" and newlines inside strings as \n.

Schema:
{
  "pages": [
    {"number": 1, "name": "page-1", "text": "brief source notes for inspection", "unclear_count": 0}
  ],
  "questions": [
    {
      "id": "q1",
      "label": "Q.1",
      "title": "exact visible question prompt if present",
      "source_pages": [1, 2],
      "answer_markdown": "Full visible answer text preserving bullets, diagrams, flowcharts, maps, examples, and [unclear] markers",
      "dimensions": {
        "introduction": "intro quality and pattern",
        "outro": "conclusion quality and pattern",
        "transition": "flow between parts",
        "diagram": "diagram/flowchart/map usage",
        "fact": "facts, examples, committees, schemes, articles, data",
        "fact_usage": "whether facts support arguments or are dumped",
        "custom": "other scoring patterns"
      }
    }
  ],
  "report": "Markdown report with overall strengths, weak spots, repeated patterns, question-wise scoring cues, and action checklist"
}

Rules:
1. Extract every visible question/answer block. Do not invent official model answers.
2. Keep pages[].text concise; answer_markdown must carry the complete visible answer.
3. Preserve structure: bullets, headings, arrows, boxes, diagrams as text labels, and evaluator marks.
4. Mark unreadable words as [unclear].
5. If a field is uncertain, keep it empty instead of guessing.

PDF name: ` + pdfName
}

func parseOneShotPDFManifest(content string, _ string) ([]Page, []Question, string, error) {
	jsonText, err := extractQuestionSplitJSON(content)
	if err != nil {
		return nil, nil, "", err
	}
	var payload oneShotPDFManifest
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, nil, "", err
	}

	pages := normalizeManifestPages(payload.Pages)
	questions := normalizeManifestQuestions(payload.Questions)
	if len(questions) == 0 {
		return nil, nil, "", errors.New("direct PDF response returned no usable question answers")
	}
	if len(pages) == 0 {
		pages = pagesFromQuestionSources(questions)
	}
	report := strings.TrimSpace(payload.Report)
	if report == "" {
		return nil, nil, "", errors.New("direct PDF response returned an empty report")
	}
	return pages, questions, report, nil
}

func normalizeManifestPages(in []oneShotPDFPage) []Page {
	pages := make([]Page, 0, len(in))
	seen := map[int]bool{}
	for i, page := range in {
		number := page.Number
		if number <= 0 {
			number = i + 1
		}
		if seen[number] {
			continue
		}
		seen[number] = true
		name := strings.TrimSpace(page.Name)
		if name == "" {
			name = fmt.Sprintf("page-%d", number)
		}
		pages = append(pages, Page{
			Number:       number,
			Name:         name,
			Text:         strings.TrimSpace(page.Text),
			UnclearCount: nonNegative(page.UnclearCount),
			Verified:     false,
		})
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Number < pages[j].Number })
	return pages
}

func normalizeManifestQuestions(in []oneShotPDFQuestion) []Question {
	questions := make([]Question, 0, len(in))
	seenIDs := map[string]int{}
	for _, item := range in {
		answer := strings.TrimSpace(item.AnswerMarkdown)
		if answer == "" {
			continue
		}
		label := firstNonBlank(item.Label, item.ID, fmt.Sprintf("Question %d", len(questions)+1))
		id := normalizeQuestionLabel(firstNonBlank(item.ID, label))
		if id == "" {
			id = fmt.Sprintf("question-%d", len(questions)+1)
		}
		seenIDs[id]++
		if seenIDs[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seenIDs[id])
		}
		question := Question{
			ID:             id,
			Label:          label,
			Title:          strings.TrimSpace(item.Title),
			SourcePages:    positiveUniqueInts(item.SourcePages),
			AnswerMarkdown: answer,
			Status:         "detected",
			Dimensions:     nonEmptyDimensions(item.Dimensions),
		}
		questions = append(questions, question)
	}
	sortQuestions(questions)
	return questions
}

func pagesFromQuestionSources(questions []Question) []Page {
	seen := map[int]bool{}
	pages := []Page{}
	for _, question := range questions {
		for _, number := range question.SourcePages {
			if seen[number] {
				continue
			}
			seen[number] = true
			pages = append(pages, Page{Number: number, Name: fmt.Sprintf("page-%d", number)})
		}
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Number < pages[j].Number })
	return pages
}

func nonEmptyDimensions(dim QuestionDimensions) *QuestionDimensions {
	if strings.TrimSpace(dim.Introduction+dim.Outro+dim.Transition+dim.Diagram+dim.Fact+dim.FactUsage+dim.Custom) == "" {
		return nil
	}
	return &QuestionDimensions{
		Introduction: strings.TrimSpace(dim.Introduction),
		Outro:        strings.TrimSpace(dim.Outro),
		Transition:   strings.TrimSpace(dim.Transition),
		Diagram:      strings.TrimSpace(dim.Diagram),
		Fact:         strings.TrimSpace(dim.Fact),
		FactUsage:    strings.TrimSpace(dim.FactUsage),
		Custom:       strings.TrimSpace(dim.Custom),
	}
}

func positiveUniqueInts(values []int) []int {
	out := []int{}
	seen := map[int]bool{}
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
