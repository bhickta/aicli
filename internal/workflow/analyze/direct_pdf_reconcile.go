package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
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
	Groups    []directPDFReconciliationGroup   `json:"groups"`
	Inventory directPDFReconciliationInventory `json:"inventory"`
	Warnings  []string                         `json:"warnings"`
	Report    directPDFReconciliationReport    `json:"report"`
}

type directPDFReconciliationGroup struct {
	ID                   string   `json:"id"`
	Status               string   `json:"status"`
	CandidateIDs         []string `json:"candidate_ids"`
	CanonicalCandidateID string   `json:"canonical_candidate_id"`
	Label                string   `json:"label"`
	Title                string   `json:"title"`
	SourcePages          []int    `json:"source_pages"`
	MergedAnswerMarkdown string   `json:"merged_answer_markdown"`
	Confidence           float64  `json:"confidence"`
	Reason               string   `json:"reason"`
}

type directPDFReconciliationInventory struct {
	VisibleQuestionSlots int `json:"visible_question_slots"`
	Answered             int `json:"answered"`
	Unanswered           int `json:"unanswered"`
}

type directPDFReconciliationReport struct {
	CopyProfile                   string                              `json:"copy_profile"`
	ScorecardSynthesis            string                              `json:"scorecard_synthesis"`
	AnswerAnalyses                []directPDFReconciliationAnswerNote `json:"answer_analyses"`
	RepeatedWinningPatterns       string                              `json:"repeated_winning_patterns"`
	WhatNotToCopyBlindly          string                              `json:"what_not_to_copy_blindly"`
	GapMap                        string                              `json:"gap_map"`
	ReusableAnswerWritingPlaybook string                              `json:"reusable_answer_writing_playbook"`
	DeliberatePracticePlan        string                              `json:"deliberate_practice_plan"`
}

type directPDFReconciliationAnswerNote struct {
	GroupID  string `json:"group_id"`
	Analysis string `json:"analysis"`
}

const (
	directPDFQuestionAnswered   = "answered"
	directPDFQuestionUnanswered = "unanswered"
)

func (s *Service) reconcileDirectPDFChunks(
	ctx context.Context,
	model string,
	pdfName string,
	sourceData []byte,
	pageCount int,
	results []directPDFChunkResult,
) ([]Question, string, []string, *provider.TokenUsage, int, error) {
	processor, ok := s.ocrProvider.(provider.DocumentProcessor)
	if !ok {
		return nil, "", nil, nil, 0, fmt.Errorf("OCR provider %q does not support direct PDF reconciliation", providerID(s.ocrProvider))
	}
	candidates := directPDFCandidates(results)
	candidateJSON, err := json.Marshal(candidates)
	if err != nil {
		return nil, "", nil, nil, 0, err
	}
	reconciliationSchema := directPDFReconciliationJSONSchema(pageCount, len(candidates))
	schemaJSON, err := json.Marshal(reconciliationSchema.Schema)
	if err != nil {
		return nil, "", nil, nil, 0, err
	}
	prompts := []string{
		directPDFReconciliationPrompt(pdfName, string(candidateJSON), string(schemaJSON), pageCount, false),
		directPDFReconciliationPrompt(pdfName, string(candidateJSON), string(schemaJSON), pageCount, true),
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
			questions, warnings, err = applyDirectPDFReconciliation(candidates, plan, pageCount)
			if err == nil {
				var report string
				report, err = buildDirectPDFReconciliationReport(questions, plan)
				if err == nil {
					warnings = cleanStringList(append(warnings, plan.Warnings...))
					return questions, report, warnings, usage, attempt + 1, nil
				}
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

func directPDFReconciliationPrompt(pdfName string, candidateJSON string, schemaJSON string, pageCount int, strict bool) string {
	prefix := ""
	if strict {
		prefix = `Your previous reconciliation was rejected because it omitted, duplicated, or invalidly grouped candidates.
Before answering, make a private checklist of every candidate ID. Assign every ID exactly once and verify the checklist before returning JSON.

`
	}
	return prefix + fmt.Sprintf(`Reconcile overlapping chunk analyses for one UPSC/Mains topper answer-copy.
The complete original PDF is attached for semantic and page-continuity verification. Candidate answer blocks extracted from overlapping chunks are included below.

Decide grouping by visible meaning, answer continuity, page order, and the attached PDF—not by brittle label-string patterns. Labels can be missing, repeated, or OCR-imperfect.

Return only one JSON object conforming to the supplied JSON Schema. Do not add prose or Markdown fences.

Rules:
1. Scan all %d PDF pages first. Create one group for every distinct visible numbered/lettered question slot, including a printed prompt whose answer area is blank.
2. Set status to "answered" only when visible candidate answer content exists. Set status to "unanswered" when the prompt is visible but its answer area is blank.
3. Assign every supplied candidate ID exactly once. Never invent or discard a candidate. An unanswered group may have no candidate IDs when chunk extraction missed the blank slot; it may also contain candidates that only describe the blank slot.
4. Every group needs a unique stable semantic id, the best visible label/title, and exact unique global source_pages between 1 and %d. Do not derive grouping from label spelling alone.
5. For answered groups, candidate_ids must be non-empty and canonical_candidate_id must name the most complete candidate in that group. Leave merged_answer_markdown empty when it covers every group page; otherwise reconstruct the complete visible answer from the PDF without summarizing or inventing content.
6. For unanswered groups, canonical_candidate_id and merged_answer_markdown must be empty. Preserve the visible prompt and blank source pages; do not invent an answer or analysis.
7. inventory counts must exactly match groups and their statuses. Check the arithmetic before returning JSON.
8. report.answer_analyses must contain exactly one entry for every answered group id and none for unanswered groups. Base each analysis on visible evidence and cite source pages.
9. The other report fields are qualitative section bodies only. Do not repeat question-slot, answered, or unanswered counts there; the application renders the validated inventory deterministically.
10. Confidence is 0-1. Add a warning for uncertain grouping, unreadable boundaries, or potentially incomplete edge-spanning answers.
11. Do not predict official UPSC marks or invent an official model answer.

PDF name: %s

JSON Schema:
%s

Candidates:
%s`, pageCount, pageCount, pdfName, schemaJSON, candidateJSON)
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
	return plan, nil
}

func applyDirectPDFReconciliation(
	candidates []directPDFCandidate,
	plan directPDFReconciliation,
	pageCount int,
) ([]Question, []string, error) {
	if pageCount <= 0 {
		return nil, nil, errors.New("direct PDF reconciliation requires a positive page count")
	}
	if err := validateDirectPDFCandidates(candidates, pageCount); err != nil {
		return nil, nil, err
	}
	byID := directPDFCandidatesByID(candidates)
	assigned := make(map[string]bool, len(candidates))
	questions := make([]Question, 0, len(plan.Groups))
	warnings := []string{}
	seenQuestionIDs := map[string]bool{}
	for groupIndex, group := range plan.Groups {
		normalizedGroup, err := validateDirectPDFReconciliationGroup(group, groupIndex, pageCount, seenQuestionIDs)
		if err != nil {
			return nil, nil, err
		}
		groupCandidates, normalizedGroup, err := assignDirectPDFGroupCandidates(byID, assigned, groupIndex, normalizedGroup)
		if err != nil {
			return nil, nil, err
		}
		question, warning, err := buildDirectPDFReconciledQuestion(byID, normalizedGroup, groupCandidates, groupIndex)
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
	if err := validateDirectPDFReconciliationInventory(plan.Inventory, questions); err != nil {
		return nil, nil, err
	}
	sortQuestions(questions)
	return questions, warnings, nil
}

func validateDirectPDFCandidates(candidates []directPDFCandidate, pageCount int) error {
	seen := make(map[string]bool, len(candidates))
	for index, candidate := range candidates {
		id := strings.TrimSpace(candidate.ID)
		if id == "" {
			return fmt.Errorf("direct PDF candidate %d has an empty id", index+1)
		}
		if seen[id] {
			return fmt.Errorf("direct PDF candidate id %q is not unique", id)
		}
		seen[id] = true
		if _, err := validateDirectPDFSourcePages(candidate.SourcePages, pageCount); err != nil {
			return fmt.Errorf("direct PDF candidate %q: %w", id, err)
		}
	}
	return nil
}

func directPDFCandidatesByID(candidates []directPDFCandidate) map[string]directPDFCandidate {
	byID := make(map[string]directPDFCandidate, len(candidates))
	for _, candidate := range candidates {
		byID[candidate.ID] = candidate
	}
	return byID
}

func validateDirectPDFReconciliationGroup(
	group directPDFReconciliationGroup,
	groupIndex int,
	pageCount int,
	seenQuestionIDs map[string]bool,
) (directPDFReconciliationGroup, error) {
	group.ID = strings.TrimSpace(group.ID)
	if group.ID == "" {
		return group, fmt.Errorf("direct PDF reconciliation group %d has an empty id", groupIndex+1)
	}
	idKey := strings.ToLower(group.ID)
	if seenQuestionIDs[idKey] {
		return group, fmt.Errorf("direct PDF reconciliation group id %q is not unique", group.ID)
	}
	seenQuestionIDs[idKey] = true

	group.Status = strings.ToLower(strings.TrimSpace(group.Status))
	if group.Status != directPDFQuestionAnswered && group.Status != directPDFQuestionUnanswered {
		return group, fmt.Errorf("direct PDF reconciliation group %q has invalid status %q", group.ID, group.Status)
	}
	group.Label = strings.TrimSpace(group.Label)
	group.Title = strings.TrimSpace(group.Title)
	if group.Label == "" && group.Title == "" {
		return group, fmt.Errorf("direct PDF reconciliation group %q has no visible label or title", group.ID)
	}

	pages, err := validateDirectPDFSourcePages(group.SourcePages, pageCount)
	if err != nil {
		return group, fmt.Errorf("direct PDF reconciliation group %q: %w", group.ID, err)
	}
	group.SourcePages = pages
	ids, err := validateDirectPDFCandidateIDs(group.CandidateIDs)
	if err != nil {
		return group, fmt.Errorf("direct PDF reconciliation group %q: %w", group.ID, err)
	}
	group.CandidateIDs = ids
	group.CanonicalCandidateID = strings.TrimSpace(group.CanonicalCandidateID)
	group.MergedAnswerMarkdown = strings.TrimSpace(group.MergedAnswerMarkdown)
	group.Reason = strings.TrimSpace(group.Reason)
	if group.Reason == "" {
		return group, fmt.Errorf("direct PDF reconciliation group %q has no evidence reason", group.ID)
	}
	if math.IsNaN(group.Confidence) || math.IsInf(group.Confidence, 0) || group.Confidence < 0 || group.Confidence > 1 {
		return group, fmt.Errorf("direct PDF reconciliation group %q has invalid confidence %v", group.ID, group.Confidence)
	}

	if group.Status == directPDFQuestionAnswered && len(group.CandidateIDs) == 0 {
		return group, fmt.Errorf("direct PDF reconciliation answered group %q has no candidates", group.ID)
	}
	if group.Status == directPDFQuestionUnanswered && (group.CanonicalCandidateID != "" || group.MergedAnswerMarkdown != "") {
		return group, fmt.Errorf("direct PDF reconciliation unanswered group %q must not contain an answer", group.ID)
	}
	return group, nil
}

func validateDirectPDFSourcePages(pages []int, pageCount int) ([]int, error) {
	if len(pages) == 0 {
		return nil, errors.New("has no source pages")
	}
	out := make([]int, len(pages))
	seen := make(map[int]bool, len(pages))
	for index, page := range pages {
		if page < 1 || page > pageCount {
			return nil, fmt.Errorf("source page %d is outside 1-%d", page, pageCount)
		}
		if seen[page] {
			return nil, fmt.Errorf("source page %d is duplicated", page)
		}
		seen[page] = true
		out[index] = page
	}
	sort.Ints(out)
	return out, nil
}

func validateDirectPDFCandidateIDs(ids []string) ([]string, error) {
	out := make([]string, len(ids))
	seen := make(map[string]bool, len(ids))
	for index, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			return nil, errors.New("contains an empty candidate id")
		}
		if seen[id] {
			return nil, fmt.Errorf("contains duplicate candidate %q", id)
		}
		seen[id] = true
		out[index] = id
	}
	return out, nil
}

func validateDirectPDFReconciliationInventory(inventory directPDFReconciliationInventory, questions []Question) error {
	answered := 0
	unanswered := 0
	for _, question := range questions {
		switch strings.ToLower(strings.TrimSpace(question.Status)) {
		case directPDFQuestionAnswered:
			answered++
		case directPDFQuestionUnanswered:
			unanswered++
		default:
			return fmt.Errorf("direct PDF reconciled question %q has invalid status %q", question.ID, question.Status)
		}
	}
	if inventory.VisibleQuestionSlots != len(questions) || inventory.Answered != answered || inventory.Unanswered != unanswered {
		return fmt.Errorf(
			"direct PDF reconciliation inventory mismatch: got total=%d answered=%d unanswered=%d; want total=%d answered=%d unanswered=%d",
			inventory.VisibleQuestionSlots,
			inventory.Answered,
			inventory.Unanswered,
			len(questions),
			answered,
			unanswered,
		)
	}
	return nil
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
) (Question, string, error) {
	allPages := directPDFCandidatePages(groupCandidates)
	allPages = append(allPages, group.SourcePages...)
	allPages = positiveUniqueInts(allPages)
	sort.Ints(allPages)
	if group.Status == directPDFQuestionUnanswered {
		question := Question{
			ID:                 group.ID,
			Label:              firstNonBlank(group.Label, group.ID),
			Title:              group.Title,
			SourcePages:        allPages,
			Status:             directPDFQuestionUnanswered,
			Boundary:           questionBoundaryNew,
			BoundaryConfidence: group.Confidence,
		}
		if len(groupCandidates) > 0 {
			canonical := selectDirectPDFCanonicalCandidate(groupCandidates)
			question.Label = firstNonBlank(group.Label, canonical.question.Label, group.ID)
			question.Title = firstNonBlank(group.Title, canonical.question.Title)
			question.Metadata = mergeDirectPDFQuestionMetadata(canonical, groupCandidates)
		}
		return question, directPDFReconciliationConfidenceWarning(question, group.Confidence), nil
	}

	canonicalID := strings.TrimSpace(group.CanonicalCandidateID)
	canonical, found := byID[canonicalID]
	if !found || !containsString(group.CandidateIDs, canonicalID) {
		return Question{}, "", fmt.Errorf("direct PDF reconciliation answered group %q has invalid canonical candidate %q", group.ID, canonicalID)
	}
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
	label := firstNonBlank(group.Label, canonical.question.Label, group.ID)
	question := canonical.question
	question.ID = group.ID
	question.Label = label
	question.Title = firstNonBlank(group.Title, canonical.question.Title)
	question.SourcePages = allPages
	question.AnswerMarkdown = answer
	question.Status = directPDFQuestionAnswered
	question.Boundary = questionBoundaryNew
	question.BoundaryConfidence = group.Confidence
	question.Dimensions = mergeDirectPDFDimensions(canonical, groupCandidates)
	question.Metadata = mergeDirectPDFQuestionMetadata(canonical, groupCandidates)
	return question, directPDFReconciliationConfidenceWarning(question, group.Confidence), nil
}

func directPDFReconciliationConfidenceWarning(question Question, confidence float64) string {
	if confidence >= 0.75 {
		return ""
	}
	return fmt.Sprintf("Question slot %q has low reconciliation confidence %.2f and requires review.", question.Label, confidence)
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
