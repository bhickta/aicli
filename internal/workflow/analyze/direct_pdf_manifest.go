package analyze

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type incompleteDirectPDFError struct {
	message string
}

func (e *incompleteDirectPDFError) Error() string {
	return e.message
}

func newIncompleteDirectPDFError(format string, args ...any) error {
	return &incompleteDirectPDFError{message: fmt.Sprintf(format, args...)}
}

func isIncompleteDirectPDFError(err error) bool {
	var target *incompleteDirectPDFError
	return errors.As(err, &target)
}

type oneShotPDFManifest struct {
	Metadata          CopyMetadata         `json:"metadata"`
	DetectedQuestions []string             `json:"detected_questions"`
	Pages             []oneShotPDFPage     `json:"pages"`
	Questions         []oneShotPDFQuestion `json:"questions"`
	Report            string               `json:"report"`
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
	Metadata       QuestionMetadata   `json:"metadata"`
}

func oneShotPDFPrompt(pdfName string) string {
	return oneShotPDFPromptBody(pdfName, false)
}

func oneShotPDFRetryPrompt(pdfName string) string {
	return oneShotPDFPromptBody(pdfName, true)
}

func oneShotPDFPromptBody(pdfName string, strictCoverage bool) string {
	prefix := ""
	if strictCoverage {
		prefix = `Your first priority is complete coverage across the whole answer-copy.
Scan the full PDF first. Build detected_questions as your coverage ledger: one entry for each distinct visible question/answer block.
Then return one questions[] object for every detected_questions entry. Do not stop at the first answer.

`
	}
	return prefix + `Analyze this entire UPSC/Mains topper answer-copy PDF with Gemini Flash-Lite.
This may come from any coaching institute or test series. Ignore institute branding/layout differences and focus on visible answer blocks.
Extract page source notes, every question with full answer text, per-question dimensions, and one final copy-level report.

Return valid JSON only. No markdown fences, no trailing commas, no prose outside JSON.
Escape double quotes inside strings as \" and newlines inside strings as \n.
Metadata must be concise. Do not shorten, summarize, or omit answer_markdown to make room for metadata.

Schema:
{
  "metadata": {
    "suggested_pdf_name": "clean searchable filename, e.g. Topper Name - GS2 - Test Code - 2020.pdf",
    "topper_name": "visible topper/candidate name if present",
    "candidate_name": "visible candidate name if distinct",
    "rank": "visible AIR/rank if present",
    "exam": "UPSC CSE / State PCS / other if visible",
    "year": "exam/test year if visible",
    "paper": "GS1 / GS2 / GS3 / GS4 / Essay / Optional / other if visible",
    "subject": "broad subject if visible",
    "test_series": "test series name if visible",
    "coaching_institute": "visible institute name if present",
    "test_code": "visible test code if present",
    "test_date": "visible copy/test date if present",
    "language": "English / Hindi / mixed / other",
    "tags": ["short search tags"],
    "search_hints": ["alternate names, paper aliases, institute aliases"],
    "notes": "short metadata confidence notes"
  },
  "detected_questions": ["visible label for answer block 1", "visible label for answer block 2"],
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
      },
      "metadata": {
        "subject": "Polity / Economy / History / Geography / Ethics / Essay / Optional / other",
        "topic": "specific topic",
        "subtopic": "narrow subtopic if visible or clear from the answer",
        "syllabus_area": "UPSC syllabus area if identifiable",
        "paper": "GS1 / GS2 / GS3 / GS4 / Essay / Optional / other",
        "question_type": "discuss / analyze / evaluate / comment / enumerate / case study / other",
        "demand": "core demand of the question",
        "difficulty": "easy / moderate / hard",
        "marks": 10,
        "word_limit": 150,
        "tags": ["short searchable tags"],
        "search_hints": ["alternate topic names"]
      }
    }
  ],
  "report": "Markdown report with overall strengths, weak spots, repeated patterns, question-wise scoring cues, and action checklist"
}

Rules:
1. First identify copy-level metadata concisely. Use only visible evidence or strong document-level inference; keep uncertain fields empty.
2. First identify every distinct visible question/answer block in detected_questions. This is mandatory coverage accounting.
3. Extract every detected question/answer block into questions[]. Do not invent official model answers.
4. Add concise metadata for every question so it can be searched/filtered by subject, topic, syllabus area, marks, word limit, and demand.
5. Do not include continuation pages separately in detected_questions. A continued page belongs to the same question.
6. Keep pages[].text concise; answer_markdown must carry the complete visible answer.
7. Preserve structure: bullets, headings, arrows, boxes, diagrams as text labels, and evaluator marks.
8. Mark unreadable words as [unclear].
9. If a field is uncertain, keep it empty instead of guessing.

PDF name: ` + pdfName
}

func parseOneShotPDFManifest(content string, _ string) (*CopyMetadata, []Page, []Question, string, error) {
	jsonText, err := extractQuestionSplitJSON(content)
	if err != nil {
		return nil, nil, nil, "", err
	}
	var payload oneShotPDFManifest
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return nil, nil, nil, "", err
	}

	pages := normalizeManifestPages(payload.Pages)
	questions := normalizeManifestQuestions(payload.Questions)
	detectedQuestions := normalizeDetectedQuestionLabels(payload.DetectedQuestions)
	if len(questions) == 0 {
		return nil, nil, nil, "", newIncompleteDirectPDFError("direct PDF response returned no usable question answers")
	}
	if len(pages) == 0 {
		pages = pagesFromQuestionSources(questions)
	}
	if err := validateManifestQuestionCoverage(detectedQuestions, questions, len(pages)); err != nil {
		return nil, nil, nil, "", err
	}
	report := strings.TrimSpace(payload.Report)
	if report == "" {
		return nil, nil, nil, "", errors.New("direct PDF response returned an empty report")
	}
	return nonEmptyCopyMetadata(payload.Metadata), pages, questions, report, nil
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
			Metadata:       nonEmptyQuestionMetadata(item.Metadata),
		}
		questions = append(questions, question)
	}
	sortQuestions(questions)
	return questions
}

func nonEmptyCopyMetadata(meta CopyMetadata) *CopyMetadata {
	meta = CopyMetadata{
		SuggestedPDFName:  strings.TrimSpace(meta.SuggestedPDFName),
		TopperName:        strings.TrimSpace(meta.TopperName),
		CandidateName:     strings.TrimSpace(meta.CandidateName),
		Rank:              strings.TrimSpace(meta.Rank),
		Exam:              strings.TrimSpace(meta.Exam),
		Year:              strings.TrimSpace(meta.Year),
		Paper:             strings.TrimSpace(meta.Paper),
		Subject:           strings.TrimSpace(meta.Subject),
		TestSeries:        strings.TrimSpace(meta.TestSeries),
		CoachingInstitute: strings.TrimSpace(meta.CoachingInstitute),
		TestCode:          strings.TrimSpace(meta.TestCode),
		TestDate:          strings.TrimSpace(meta.TestDate),
		Language:          strings.TrimSpace(meta.Language),
		Tags:              cleanStringList(meta.Tags),
		SearchHints:       cleanStringList(meta.SearchHints),
		Notes:             strings.TrimSpace(meta.Notes),
	}
	if meta.SuggestedPDFName+meta.TopperName+meta.CandidateName+meta.Rank+meta.Exam+meta.Year+meta.Paper+meta.Subject+meta.TestSeries+meta.CoachingInstitute+meta.TestCode+meta.TestDate+meta.Language+strings.Join(meta.Tags, "")+strings.Join(meta.SearchHints, "")+meta.Notes == "" {
		return nil
	}
	return &meta
}

func nonEmptyQuestionMetadata(meta QuestionMetadata) *QuestionMetadata {
	meta = QuestionMetadata{
		Subject:      strings.TrimSpace(meta.Subject),
		Topic:        strings.TrimSpace(meta.Topic),
		Subtopic:     strings.TrimSpace(meta.Subtopic),
		SyllabusArea: strings.TrimSpace(meta.SyllabusArea),
		Paper:        strings.TrimSpace(meta.Paper),
		QuestionType: strings.TrimSpace(meta.QuestionType),
		Demand:       strings.TrimSpace(meta.Demand),
		Difficulty:   strings.TrimSpace(meta.Difficulty),
		Marks:        nonNegative(meta.Marks),
		WordLimit:    nonNegative(meta.WordLimit),
		Tags:         cleanStringList(meta.Tags),
		SearchHints:  cleanStringList(meta.SearchHints),
	}
	if meta.Subject+meta.Topic+meta.Subtopic+meta.SyllabusArea+meta.Paper+meta.QuestionType+meta.Demand+meta.Difficulty+strings.Join(meta.Tags, "")+strings.Join(meta.SearchHints, "") == "" && meta.Marks == 0 && meta.WordLimit == 0 {
		return nil
	}
	return &meta
}

func cleanStringList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func normalizeDetectedQuestionLabels(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, label := range in {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, label)
	}
	return out
}

func validateManifestQuestionCoverage(detectedQuestions []string, questions []Question, pageCount int) error {
	expected := len(detectedQuestions)
	if expected == 0 {
		if pageCount >= 6 {
			return newIncompleteDirectPDFError(
				"direct PDF response did not include detected_questions coverage list for %d page(s); refusing to save partial result",
				pageCount,
			)
		}
		return nil
	}
	extracted := len(questions)
	if expected < 2 || extracted >= expected || extracted*100 >= expected*70 {
		return nil
	}
	return newIncompleteDirectPDFError(
		"direct PDF response is incomplete: detected %d question/answer block(s), but returned %d question answer block(s); refusing to save partial result",
		expected,
		extracted,
	)
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
