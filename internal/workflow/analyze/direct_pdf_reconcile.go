package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

type directPDFCandidate struct {
	ID             string `json:"id"`
	Chunk          int    `json:"chunk"`
	ChunkFirstPage int    `json:"chunk_first_page"`
	ChunkLastPage  int    `json:"chunk_last_page"`
	Label          string `json:"label"`
	Title          string `json:"title"`
	SourcePages    []int  `json:"source_pages"`
	AnswerMarkdown string `json:"answer_markdown"`
	question       Question
}

type directPDFReconciliation struct {
	Groups   []directPDFReconciliationGroup `json:"groups"`
	Warnings []string                       `json:"warnings"`
	Report   string                         `json:"report"`
}

type directPDFReconciliationGroup struct {
	ID                   string   `json:"id"`
	CandidateIDs         []string `json:"candidate_ids"`
	CanonicalCandidateID string   `json:"canonical_candidate_id"`
	Label                string   `json:"label"`
	Title                string   `json:"title"`
	MergedAnswerMarkdown string   `json:"merged_answer_markdown"`
	Confidence           float64  `json:"confidence"`
	Reason               string   `json:"reason"`
}

func (s *Service) reconcileDirectPDFChunks(
	ctx context.Context,
	model string,
	pdfName string,
	sourceData []byte,
	results []directPDFChunkResult,
) ([]Question, string, []string, *provider.TokenUsage, int, error) {
	processor, ok := s.ocrProvider.(provider.DocumentProcessor)
	if !ok {
		return nil, "", nil, nil, 0, fmt.Errorf("OCR provider %q does not support direct PDF reconciliation", providerID(s.ocrProvider))
	}
	candidates := directPDFCandidates(results)
	if len(candidates) == 0 {
		return nil, "", nil, nil, 0, errors.New("direct PDF chunks returned no answer candidates")
	}
	candidateJSON, err := json.Marshal(candidates)
	if err != nil {
		return nil, "", nil, nil, 0, err
	}
	prompts := []string{
		directPDFReconciliationPrompt(pdfName, string(candidateJSON), false),
		directPDFReconciliationPrompt(pdfName, string(candidateJSON), true),
	}
	var usage *provider.TokenUsage
	var lastErr error
	for attempt, prompt := range prompts {
		res, err := processor.Document(ctx, provider.DocumentRequest{
			Model:            model,
			Prompt:           prompt,
			Data:             sourceData,
			MIMEType:         "application/pdf",
			ResponseMIMEType: "application/json",
			Temperature:      0,
			MaxTokens:        geminiLiteDirectPDFMaxTokens,
		})
		usage = addTokenUsage(usage, res.Usage)
		if err != nil {
			return nil, "", nil, usage, attempt + 1, err
		}
		if err := validateDirectPDFFinishReason(res.FinishReason); err != nil {
			return nil, "", nil, usage, attempt + 1, err
		}
		plan, err := parseDirectPDFReconciliation(res.Content)
		if err == nil {
			var questions []Question
			var warnings []string
			questions, warnings, err = applyDirectPDFReconciliation(candidates, plan)
			if err == nil {
				warnings = cleanStringList(append(warnings, plan.Warnings...))
				return questions, strings.TrimSpace(plan.Report), warnings, usage, attempt + 1, nil
			}
		}
		lastErr = err
		if attempt+1 < len(prompts) {
			s.logWarn("direct PDF reconciliation invalid; retrying with strict coverage", "attempt", attempt+1, "error", err)
			continue
		}
	}
	return nil, "", nil, usage, len(prompts), lastErr
}

func directPDFCandidates(results []directPDFChunkResult) []directPDFCandidate {
	candidates := []directPDFCandidate{}
	for _, result := range results {
		for questionIndex, question := range result.Response.Questions {
			candidates = append(candidates, directPDFCandidate{
				ID:             fmt.Sprintf("chunk-%03d-question-%03d", result.Chunk.Index+1, questionIndex+1),
				Chunk:          result.Chunk.Index + 1,
				ChunkFirstPage: result.Chunk.FirstPage,
				ChunkLastPage:  result.Chunk.LastPage,
				Label:          question.Label,
				Title:          question.Title,
				SourcePages:    append([]int(nil), question.SourcePages...),
				AnswerMarkdown: question.AnswerMarkdown,
				question:       question,
			})
		}
	}
	return candidates
}

func directPDFReconciliationPrompt(pdfName string, candidateJSON string, strict bool) string {
	prefix := ""
	if strict {
		prefix = `Your previous reconciliation was rejected because it omitted, duplicated, or invalidly grouped candidates.
Before answering, make a private checklist of every candidate ID. Assign every ID exactly once and verify the checklist before returning JSON.

`
	}
	return prefix + `Reconcile overlapping chunk analyses for one UPSC/Mains topper answer-copy.
The complete original PDF is attached for semantic and page-continuity verification. Candidate answer blocks extracted from overlapping chunks are included below.

Decide grouping by visible meaning, answer continuity, page order, and the attached PDF—not by brittle label-string patterns. Labels can be missing, repeated, or OCR-imperfect.

Return valid JSON only with this schema:
{
  "groups": [
    {
      "id": "stable semantic group id",
      "candidate_ids": ["every duplicate/continuation candidate for this one answer"],
      "canonical_candidate_id": "the most complete, accurate candidate in candidate_ids",
      "label": "best visible answer label",
      "title": "best exact visible question prompt, if present",
      "merged_answer_markdown": "",
      "confidence": 0.0,
      "reason": "short semantic/page-continuity reason"
    }
  ],
  "warnings": ["specific unresolved uncertainty requiring human review"],
  "report": "copy-wide evidence-based Markdown report"
}

Rules:
1. Assign every candidate ID exactly once. Never invent an ID and never discard a candidate.
2. Put duplicate extractions of the same visible answer in one group. Keep adjacent but distinct questions/subparts in different groups.
3. Put semantic continuations in the same group even when their visible headings or labels differ.
4. Choose a canonical candidate that contains the most complete visible answer and strongest evidence-grounded analysis.
5. Leave merged_answer_markdown empty when one candidate fully covers the group's source pages. If no single candidate covers all group source pages, reconstruct the complete visible answer in merged_answer_markdown using only the attached PDF and candidate text. Do not summarize or invent content.
6. Confidence is 0-1. Add a warning for uncertain grouping, unreadable boundaries, or potentially incomplete edge-spanning answers.
7. The report must synthesize the whole copy across all final answer groups: Copy Profile, Scorecard Synthesis, Answer-Wise Analysis, Repeated Winning Patterns, What Not to Copy Blindly, Gap Map, Reusable Answer-Writing Playbook, and Deliberate-Practice Plan. Cite answer labels and pages; distinguish observation, interpretation, and recommendation.
8. Do not predict official UPSC marks or invent an official model answer.

PDF name: ` + pdfName + `

Candidates:
` + candidateJSON
}

func parseDirectPDFReconciliation(content string) (directPDFReconciliation, error) {
	jsonText, err := extractQuestionSplitJSON(content)
	if err != nil {
		return directPDFReconciliation{}, err
	}
	var plan directPDFReconciliation
	if err := json.Unmarshal([]byte(jsonText), &plan); err != nil {
		return directPDFReconciliation{}, err
	}
	if len(plan.Groups) == 0 {
		return directPDFReconciliation{}, errors.New("direct PDF reconciliation returned no groups")
	}
	if strings.TrimSpace(plan.Report) == "" {
		return directPDFReconciliation{}, errors.New("direct PDF reconciliation returned an empty report")
	}
	return plan, nil
}

func applyDirectPDFReconciliation(
	candidates []directPDFCandidate,
	plan directPDFReconciliation,
) ([]Question, []string, error) {
	byID := directPDFCandidatesByID(candidates)
	assigned := make(map[string]bool, len(candidates))
	questions := make([]Question, 0, len(plan.Groups))
	warnings := []string{}
	seenQuestionIDs := map[string]int{}
	for groupIndex, group := range plan.Groups {
		group.CandidateIDs = cleanStringList(group.CandidateIDs)
		if len(group.CandidateIDs) == 0 {
			continue
		}
		groupCandidates, normalizedGroup, err := assignDirectPDFGroupCandidates(byID, assigned, groupIndex, group)
		if err != nil {
			return nil, nil, err
		}
		question, warning, err := buildDirectPDFReconciledQuestion(byID, normalizedGroup, groupCandidates, groupIndex, seenQuestionIDs)
		if err != nil {
			return nil, nil, err
		}
		questions = append(questions, question)
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	if missing := unassignedDirectPDFCandidates(candidates, assigned); len(missing) > 0 {
		return nil, nil, fmt.Errorf("direct PDF reconciliation omitted candidate(s): %s", strings.Join(missing, ", "))
	}
	sortQuestions(questions)
	return questions, warnings, nil
}

func directPDFCandidatesByID(candidates []directPDFCandidate) map[string]directPDFCandidate {
	byID := make(map[string]directPDFCandidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	return byID
}

func assignDirectPDFGroupCandidates(
	byID map[string]directPDFCandidate,
	assigned map[string]bool,
	groupIndex int,
	group directPDFReconciliationGroup,
) ([]directPDFCandidate, directPDFReconciliationGroup, error) {
	groupCandidates := make([]directPDFCandidate, 0, len(group.CandidateIDs))
	for _, id := range group.CandidateIDs {
		candidate, found := byID[id]
		if !found {
			return nil, group, fmt.Errorf("direct PDF reconciliation group %d references unknown candidate %q", groupIndex+1, id)
		}
		if assigned[id] {
			return nil, group, fmt.Errorf("direct PDF reconciliation candidate %q was assigned more than once", id)
		}
		assigned[id] = true
		groupCandidates = append(groupCandidates, candidate)
	}
	return groupCandidates, group, nil
}

func buildDirectPDFReconciledQuestion(
	byID map[string]directPDFCandidate,
	group directPDFReconciliationGroup,
	groupCandidates []directPDFCandidate,
	groupIndex int,
	seenQuestionIDs map[string]int,
) (Question, string, error) {
	canonicalID := strings.TrimSpace(group.CanonicalCandidateID)
	canonical, found := byID[canonicalID]
	if !found || !containsString(group.CandidateIDs, canonicalID) {
		canonical = selectDirectPDFCanonicalCandidate(groupCandidates)
		canonicalID = canonical.ID
	}
	allPages := directPDFCandidatePages(groupCandidates)
	answer := strings.TrimSpace(group.MergedAnswerMarkdown)
	if answer == "" {
		if !candidateCoversPages(canonical, allPages) {
			return Question{}, "", fmt.Errorf("direct PDF reconciliation group %d spans pages %v but canonical candidate %q does not cover them and no merged answer was returned", groupIndex+1, allPages, canonicalID)
		}
		answer = strings.TrimSpace(canonical.question.AnswerMarkdown)
	}
	if answer == "" {
		return Question{}, "", fmt.Errorf("direct PDF reconciliation group %d returned an empty answer", groupIndex+1)
	}
	label := firstNonBlank(group.Label, canonical.question.Label, fmt.Sprintf("Question %d", groupIndex+1))
	questionID := normalizeQuestionLabel(firstNonBlank(group.ID, canonical.question.ID, label))
	if questionID == "" {
		questionID = fmt.Sprintf("question-%d", groupIndex+1)
	}
	seenQuestionIDs[questionID]++
	if seenQuestionIDs[questionID] > 1 {
		questionID = fmt.Sprintf("%s-%d", questionID, seenQuestionIDs[questionID])
	}
	question := canonical.question
	question.ID = questionID
	question.Label = label
	question.Title = firstNonBlank(group.Title, canonical.question.Title)
	question.SourcePages = allPages
	question.AnswerMarkdown = answer
	question.Status = "detected"
	question.Boundary = questionBoundaryNew
	question.BoundaryConfidence = clampFloat(group.Confidence, 0, 1)
	question.Dimensions = mergeDirectPDFDimensions(canonical, groupCandidates)
	question.Metadata = mergeDirectPDFQuestionMetadata(canonical, groupCandidates)
	warning := ""
	if group.Confidence < 0.75 {
		warning = fmt.Sprintf("Answer group %q has low reconciliation confidence %.2f and requires review.", label, group.Confidence)
	}
	return question, warning, nil
}

func unassignedDirectPDFCandidates(candidates []directPDFCandidate, assigned map[string]bool) []string {
	missing := make([]string, 0, len(candidates)-len(assigned))
	for _, candidate := range candidates {
		if !assigned[candidate.ID] {
			missing = append(missing, candidate.ID)
		}
	}
	return missing
}

func selectDirectPDFCanonicalCandidate(candidates []directPDFCandidate) directPDFCandidate {
	selected := candidates[0]
	for _, candidate := range candidates[1:] {
		if directPDFCandidateScore(candidate) > directPDFCandidateScore(selected) {
			selected = candidate
		}
	}
	return selected
}

func directPDFCandidateScore(candidate directPDFCandidate) int {
	score := len(candidate.SourcePages)*100000 + len(strings.TrimSpace(candidate.AnswerMarkdown))
	if !candidateTouchesChunkEdge(candidate) {
		score += 1000000
	}
	if candidate.question.Dimensions != nil {
		score += 10000
	}
	if candidate.question.Metadata != nil {
		score += 5000
	}
	return score
}

func candidateTouchesChunkEdge(candidate directPDFCandidate) bool {
	for _, page := range candidate.SourcePages {
		if page == candidate.ChunkFirstPage || page == candidate.ChunkLastPage {
			return true
		}
	}
	return false
}

func directPDFCandidatePages(candidates []directPDFCandidate) []int {
	pages := []int{}
	for _, candidate := range candidates {
		pages = append(pages, candidate.SourcePages...)
	}
	pages = positiveUniqueInts(pages)
	sort.Ints(pages)
	return pages
}

func candidateCoversPages(candidate directPDFCandidate, pages []int) bool {
	covered := make(map[int]bool, len(candidate.SourcePages))
	for _, page := range candidate.SourcePages {
		covered[page] = true
	}
	for _, page := range pages {
		if !covered[page] {
			return false
		}
	}
	return true
}

func mergeDirectPDFDimensions(canonical directPDFCandidate, candidates []directPDFCandidate) *QuestionDimensions {
	if canonical.question.Dimensions == nil {
		for _, candidate := range candidates {
			if candidate.question.Dimensions != nil {
				copy := *candidate.question.Dimensions
				return &copy
			}
		}
		return nil
	}
	copy := *canonical.question.Dimensions
	return &copy
}

func mergeDirectPDFQuestionMetadata(canonical directPDFCandidate, candidates []directPDFCandidate) *QuestionMetadata {
	var merged *QuestionMetadata
	ordered := append([]directPDFCandidate{canonical}, candidates...)
	seen := map[string]bool{}
	for _, candidate := range ordered {
		if seen[candidate.ID] || candidate.question.Metadata == nil {
			continue
		}
		seen[candidate.ID] = true
		meta := candidate.question.Metadata
		if merged == nil {
			copy := *meta
			copy.Tags = append([]string(nil), meta.Tags...)
			copy.SearchHints = append([]string(nil), meta.SearchHints...)
			merged = &copy
			continue
		}
		merged.Subject = firstNonBlank(merged.Subject, meta.Subject)
		merged.Topic = firstNonBlank(merged.Topic, meta.Topic)
		merged.Subtopic = firstNonBlank(merged.Subtopic, meta.Subtopic)
		merged.SyllabusArea = firstNonBlank(merged.SyllabusArea, meta.SyllabusArea)
		merged.Paper = firstNonBlank(merged.Paper, meta.Paper)
		merged.QuestionType = firstNonBlank(merged.QuestionType, meta.QuestionType)
		merged.Demand = firstNonBlank(merged.Demand, meta.Demand)
		merged.Difficulty = firstNonBlank(merged.Difficulty, meta.Difficulty)
		if merged.Marks == 0 {
			merged.Marks = meta.Marks
		}
		if merged.WordLimit == 0 {
			merged.WordLimit = meta.WordLimit
		}
		merged.Tags = cleanStringList(append(merged.Tags, meta.Tags...))
		merged.SearchHints = cleanStringList(append(merged.SearchHints, meta.SearchHints...))
	}
	return merged
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
