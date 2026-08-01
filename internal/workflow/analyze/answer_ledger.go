package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type answerBoundaryLedger struct {
	Decisions []answerBoundaryDecision `json:"decisions"`
}

type answerBoundaryDecision struct {
	PageNumber         int     `json:"page_number"`
	Boundary           string  `json:"boundary"`
	BoundaryConfidence float64 `json:"boundary_confidence"`
	VisibleLabel       string  `json:"visible_label"`
	LabelEvidence      string  `json:"label_evidence"`
	Reason             string  `json:"reason"`
}

type answerBoundaryLedgerPayload struct {
	Decisions []answerBoundaryDecisionPayload `json:"decisions"`
}

type answerBoundaryDecisionPayload struct {
	PageNumber         *int     `json:"page_number"`
	Boundary           *string  `json:"boundary"`
	BoundaryConfidence *float64 `json:"boundary_confidence"`
	VisibleLabel       *string  `json:"visible_label"`
	LabelEvidence      *string  `json:"label_evidence"`
	Reason             *string  `json:"reason"`
}

func (s *Service) groupAnswerPages(
	ctx context.Context,
	model string,
	pages []Page,
) ([]Question, error) {
	if len(pages) == 0 {
		return []Question{}, nil
	}
	res, err := s.questionProvider.Chat(ctx, provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: answerBoundaryLedgerPrompt(pages),
			},
		},
		Temperature:    0,
		MaxTokens:      max(1000, len(pages)*120),
		ResponseSchema: answerBoundaryLedgerJSONSchema(),
	})
	if err != nil {
		return nil, fmt.Errorf("request answer-boundary ledger: %w", err)
	}
	ledger, err := parseAnswerBoundaryLedger(res.Content)
	if err != nil {
		return nil, err
	}
	return questionsFromBoundaryLedger(pages, ledger)
}

func parseAnswerBoundaryLedger(content string) (answerBoundaryLedger, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return answerBoundaryLedger{}, errors.New("empty answer-boundary ledger response")
	}
	var payload answerBoundaryLedgerPayload
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return answerBoundaryLedger{}, fmt.Errorf("parse answer-boundary ledger: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return answerBoundaryLedger{}, errors.New("parse answer-boundary ledger: trailing content")
	}
	if len(payload.Decisions) == 0 {
		return answerBoundaryLedger{}, errors.New("answer-boundary ledger returned no page decisions")
	}
	ledger := answerBoundaryLedger{Decisions: make([]answerBoundaryDecision, 0, len(payload.Decisions))}
	for i, decision := range payload.Decisions {
		parsed, err := decision.strictValue(i)
		if err != nil {
			return answerBoundaryLedger{}, err
		}
		ledger.Decisions = append(ledger.Decisions, parsed)
	}
	return ledger, nil
}

func (d answerBoundaryDecisionPayload) strictValue(index int) (answerBoundaryDecision, error) {
	missing := ""
	switch {
	case d.PageNumber == nil:
		missing = "page_number"
	case d.Boundary == nil:
		missing = "boundary"
	case d.BoundaryConfidence == nil:
		missing = "boundary_confidence"
	case d.VisibleLabel == nil:
		missing = "visible_label"
	case d.LabelEvidence == nil:
		missing = "label_evidence"
	case d.Reason == nil:
		missing = "reason"
	}
	if missing != "" {
		return answerBoundaryDecision{}, fmt.Errorf("answer-boundary decision %d missing required field %q", index+1, missing)
	}
	decision := answerBoundaryDecision{
		PageNumber:         *d.PageNumber,
		Boundary:           *d.Boundary,
		BoundaryConfidence: *d.BoundaryConfidence,
		VisibleLabel:       *d.VisibleLabel,
		LabelEvidence:      *d.LabelEvidence,
		Reason:             *d.Reason,
	}
	if err := validateAnswerBoundaryDecision(decision, index); err != nil {
		return answerBoundaryDecision{}, err
	}
	return decision, nil
}

func validateAnswerBoundaryDecision(decision answerBoundaryDecision, index int) error {
	if decision.PageNumber < 1 || decision.PageNumber > 10000 {
		return fmt.Errorf("answer-boundary decision %d page_number %d is outside [1,10000]", index+1, decision.PageNumber)
	}
	switch decision.Boundary {
	case questionBoundaryNew, questionBoundaryContinuation, questionBoundaryUncertain:
	default:
		return fmt.Errorf("answer-boundary decision %d has invalid boundary %q", index+1, decision.Boundary)
	}
	if decision.BoundaryConfidence < 0 || decision.BoundaryConfidence > 1 {
		return fmt.Errorf(
			"answer-boundary decision %d boundary_confidence %v is outside [0,1]",
			index+1,
			decision.BoundaryConfidence,
		)
	}
	return nil
}

func questionsFromBoundaryLedger(pages []Page, ledger answerBoundaryLedger) ([]Question, error) {
	orderedPages := append([]Page(nil), pages...)
	sort.SliceStable(orderedPages, func(i, j int) bool {
		return orderedPages[i].Number < orderedPages[j].Number
	})
	pageByNumber := make(map[int]Page, len(orderedPages))
	for _, page := range orderedPages {
		if _, duplicate := pageByNumber[page.Number]; duplicate {
			return nil, fmt.Errorf("duplicate source page %d", page.Number)
		}
		pageByNumber[page.Number] = page
	}
	decisionByPage := make(map[int]answerBoundaryDecision, len(ledger.Decisions))
	for i, decision := range ledger.Decisions {
		if err := validateAnswerBoundaryDecision(decision, i); err != nil {
			return nil, err
		}
		if _, ok := pageByNumber[decision.PageNumber]; !ok {
			return nil, fmt.Errorf("answer-boundary ledger referenced unknown page %d", decision.PageNumber)
		}
		if _, duplicate := decisionByPage[decision.PageNumber]; duplicate {
			return nil, fmt.Errorf("answer-boundary ledger repeated page %d", decision.PageNumber)
		}
		decisionByPage[decision.PageNumber] = decision
	}
	if len(decisionByPage) != len(pageByNumber) {
		return nil, fmt.Errorf(
			"answer-boundary ledger covered %d of %d pages",
			len(decisionByPage),
			len(pageByNumber),
		)
	}

	blocks := make([]Question, 0, len(orderedPages))
	for _, page := range orderedPages {
		decision := decisionByPage[page.Number]
		blocks = append(blocks, questionFromBoundaryDecision(page, decision))
	}
	return mergeQuestionBlocks(blocks), nil
}

func questionFromBoundaryDecision(page Page, decision answerBoundaryDecision) Question {
	boundary := decision.Boundary
	confidence := decision.BoundaryConfidence
	label := decision.VisibleLabel
	evidence := decision.LabelEvidence
	hasLabelClaim := label != "" || evidence != ""
	status := "detected"
	labelIsGrounded := label == "" && evidence == ""
	if label != "" && evidence != "" {
		labelIsGrounded = strings.Contains(page.Text, evidence) && strings.Contains(evidence, label)
	}
	if !labelIsGrounded {
		label = ""
		boundary = questionBoundaryUncertain
	}
	if boundary == questionBoundaryContinuation && hasLabelClaim {
		boundary = questionBoundaryUncertain
	}
	if confidence < 0.7 {
		boundary = questionBoundaryUncertain
	}
	if boundary == questionBoundaryNew && label == "" {
		status = "needs review"
	}
	if boundary == questionBoundaryUncertain {
		status = "needs review"
	}
	if label == "" {
		if boundary == questionBoundaryContinuation {
			label = fmt.Sprintf("Page %d continuation", page.Number)
		} else {
			label = fmt.Sprintf("Page %d block", page.Number)
		}
	}
	return Question{
		ID:                 normalizeQuestionLabel(label),
		Label:              label,
		AnswerMarkdown:     strings.TrimSpace(page.Text),
		SourcePages:        []int{page.Number},
		Status:             status,
		Boundary:           boundary,
		BoundaryConfidence: confidence,
	}
}

func answerBoundaryLedgerPrompt(pages []Page) string {
	var prompt strings.Builder
	prompt.WriteString(`Build an ordered answer-boundary ledger for these OCR pages from one UPSC/Mains answer copy.

This is a narrow classification task. Do not rewrite, summarize, or extract the answer text.
Return exactly one decision for every supplied page using the response schema.

Boundary meanings:
- new_answer: the page clearly begins a distinct answer.
- continuation: the page continues the answer from the immediately preceding supplied page and has no distinct new-answer start.
- uncertain: the OCR evidence is insufficient or the page appears to contain multiple answer starts.

Rules:
- Use visible answer numbering, sentence carry-over, topic continuity, layout changes, and explicit answer resets together.
- Do not treat page headers, printed form boilerplate, section headings, bullet headings, or answer content as answer labels.
- visible_label is the exact answer number/label only when visibly present; otherwise it is an empty string.
- label_evidence is the shortest exact OCR excerpt containing visible_label; otherwise it is an empty string.
- A change of topic alone is not enough to invent a label.
- Use uncertain instead of guessing and lower boundary_confidence when evidence conflicts.

Pages:
`)
	for _, page := range pages {
		fmt.Fprintf(&prompt, "\n<page number=%q>\n%s\n</page>\n", fmt.Sprint(page.Number), page.Text)
	}
	return prompt.String()
}
