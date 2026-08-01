package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/bhickta/aicli/internal/provider"
)

const maxAnalysisItems = 4

func (s *Service) extractDimensions(
	ctx context.Context,
	model string,
	questions []Question,
	workers int,
	progress func(completed, total int),
) []Question {
	if len(questions) == 0 {
		return questions
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(questions) {
		workers = len(questions)
	}

	results := make([]Question, len(questions))
	copy(results, questions)

	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	completed := 0
	var completedMu sync.Mutex
	markCompleted := func() {
		completedMu.Lock()
		defer completedMu.Unlock()
		completed++
		if progress != nil {
			progress(completed, len(questions))
		}
	}

	for index, question := range questions {
		if strings.TrimSpace(question.AnswerMarkdown) == "" || question.Status == "needs review" {
			markCompleted()
			continue
		}

		wg.Go(func() {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				markCompleted()
				return
			}
			defer markCompleted()

			res, err := s.questionProvider.Chat(ctx, provider.ChatRequest{
				Model: model,
				Messages: []provider.Message{
					{
						Role:    "user",
						Content: questionDimensionsPrompt(question),
					},
				},
				Temperature:    0,
				MaxTokens:      3200,
				ResponseSchema: questionDimensionsJSONSchema(),
			})
			if err != nil {
				s.logWarn("failed to extract question analysis", "question", question.Label, "error", err)
				return
			}

			dimensions, err := parseQuestionDimensions(res.Content)
			if err != nil {
				s.logWarn(
					"failed to parse question analysis",
					"question",
					question.Label,
					"error",
					err,
				)
				return
			}
			results[index].Dimensions = dimensions
		})
	}

	wg.Wait()
	return results
}

func parseQuestionDimensions(content string) (*QuestionDimensions, error) {
	jsonText, err := extractQuestionSplitJSON(content)
	if err != nil {
		return nil, fmt.Errorf("extract question analysis JSON: %w", err)
	}

	jsonText, err = normalizeQuestionDimensionTextFields(jsonText)
	if err != nil {
		return nil, err
	}

	var dimensions QuestionDimensions
	if err := json.Unmarshal([]byte(jsonText), &dimensions); err != nil {
		return nil, fmt.Errorf("decode question analysis: %w", err)
	}

	normalized := nonEmptyDimensions(dimensions)
	if normalized == nil {
		return nil, errors.New("question analysis is empty")
	}
	return normalized, nil
}

func normalizeQuestionDimensionTextFields(jsonText string) (string, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(jsonText), &payload); err != nil {
		return "", fmt.Errorf("decode question analysis: %w", err)
	}

	for _, key := range []string{
		"introduction",
		"outro",
		"transition",
		"diagram",
		"fact",
		"fact_usage",
		"custom",
		"demand_alignment",
		"body_structure",
		"content_depth",
		"multidimensionality",
		"presentation",
	} {
		if value, ok := payload[key]; ok {
			payload[key] = dimensionText(value)
		}
	}

	normalized, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode normalized question analysis: %w", err)
	}
	return string(normalized), nil
}

func dimensionText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := dimensionText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			if text := dimensionText(typed[key]); text != "" {
				parts = append(parts, key+": "+text)
			}
		}
		return strings.Join(parts, "; ")
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func nonEmptyDimensions(dimensions QuestionDimensions) *QuestionDimensions {
	dimensions.Introduction = strings.TrimSpace(dimensions.Introduction)
	dimensions.Outro = strings.TrimSpace(dimensions.Outro)
	dimensions.Transition = strings.TrimSpace(dimensions.Transition)
	dimensions.Diagram = strings.TrimSpace(dimensions.Diagram)
	dimensions.Fact = strings.TrimSpace(dimensions.Fact)
	dimensions.FactUsage = strings.TrimSpace(dimensions.FactUsage)
	dimensions.Custom = strings.TrimSpace(dimensions.Custom)
	dimensions.DemandAlignment = strings.TrimSpace(dimensions.DemandAlignment)
	dimensions.BodyStructure = strings.TrimSpace(dimensions.BodyStructure)
	dimensions.ContentDepth = strings.TrimSpace(dimensions.ContentDepth)
	dimensions.Multidimensionality = strings.TrimSpace(dimensions.Multidimensionality)
	dimensions.Presentation = strings.TrimSpace(dimensions.Presentation)
	dimensions.Strengths = cleanAnalysisPoints(dimensions.Strengths, maxAnalysisItems)
	dimensions.Gaps = cleanAnalysisPoints(dimensions.Gaps, maxAnalysisItems)
	dimensions.MissingDimensions = cleanStringList(dimensions.MissingDimensions)
	dimensions.ExaminerSignals = cleanStringList(dimensions.ExaminerSignals)
	dimensions.Improvements = cleanAnalysisImprovements(dimensions.Improvements, maxAnalysisItems)
	dimensions.ReusableTechniques = cleanStringList(dimensions.ReusableTechniques)
	dimensions.Scorecard = normalizeQuestionScorecard(dimensions.Scorecard)

	text := dimensions.Introduction + dimensions.Outro + dimensions.Transition + dimensions.Diagram +
		dimensions.Fact + dimensions.FactUsage + dimensions.Custom + dimensions.DemandAlignment +
		dimensions.BodyStructure + dimensions.ContentDepth + dimensions.Multidimensionality + dimensions.Presentation
	hasCollections := len(dimensions.Strengths)+len(dimensions.Gaps)+len(dimensions.MissingDimensions)+
		len(dimensions.ExaminerSignals)+len(dimensions.Improvements)+len(dimensions.ReusableTechniques) > 0
	if text == "" && !hasCollections && dimensions.Scorecard == nil {
		return nil
	}
	return &dimensions
}

func cleanAnalysisPoints(points []AnalysisPoint, limit int) []AnalysisPoint {
	cleaned := make([]AnalysisPoint, 0, min(len(points), limit))
	seen := map[string]bool{}
	for _, point := range points {
		point.Point = strings.TrimSpace(point.Point)
		if point.Point == "" {
			continue
		}
		key := strings.ToLower(point.Point)
		if seen[key] {
			continue
		}
		seen[key] = true
		point.Evidence = strings.TrimSpace(point.Evidence)
		point.WhyItMatters = strings.TrimSpace(point.WhyItMatters)
		cleaned = append(cleaned, point)
		if len(cleaned) == limit {
			break
		}
	}
	return cleaned
}

func cleanAnalysisImprovements(improvements []AnalysisImprovement, limit int) []AnalysisImprovement {
	cleaned := make([]AnalysisImprovement, 0, min(len(improvements), limit))
	seen := map[string]bool{}
	for _, improvement := range improvements {
		improvement.Change = strings.TrimSpace(improvement.Change)
		if improvement.Change == "" {
			continue
		}
		key := strings.ToLower(improvement.Change)
		if seen[key] {
			continue
		}
		seen[key] = true
		improvement.Priority = normalizePriority(improvement.Priority)
		improvement.Example = strings.TrimSpace(improvement.Example)
		cleaned = append(cleaned, improvement)
		if len(cleaned) == limit {
			break
		}
	}
	return cleaned
}

func normalizePriority(priority string) string {
	switch strings.ToLower(strings.TrimSpace(priority)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(priority))
	default:
		return "medium"
	}
}

func normalizeQuestionScorecard(scorecard *QuestionScorecard) *QuestionScorecard {
	if scorecard == nil {
		return nil
	}
	scorecard.DemandFulfilment = clamp(scorecard.DemandFulfilment, 0, 5)
	scorecard.Structure = clamp(scorecard.Structure, 0, 5)
	scorecard.ContentDepth = clamp(scorecard.ContentDepth, 0, 5)
	scorecard.Evidence = clamp(scorecard.Evidence, 0, 5)
	scorecard.Multidimensionality = clamp(scorecard.Multidimensionality, 0, 5)
	scorecard.Presentation = clamp(scorecard.Presentation, 0, 5)
	scorecard.Conclusion = clamp(scorecard.Conclusion, 0, 5)
	scorecard.OverallPercent = clamp(scorecard.OverallPercent, 0, 100)
	scorecard.EstimatedBand = strings.TrimSpace(scorecard.EstimatedBand)
	scorecard.Confidence = strings.TrimSpace(scorecard.Confidence)
	scorecard.Rationale = strings.TrimSpace(scorecard.Rationale)

	total := scorecard.DemandFulfilment + scorecard.Structure + scorecard.ContentDepth + scorecard.Evidence +
		scorecard.Multidimensionality + scorecard.Presentation + scorecard.Conclusion + scorecard.OverallPercent
	if total == 0 && scorecard.EstimatedBand+scorecard.Confidence+scorecard.Rationale == "" {
		return nil
	}
	return scorecard
}

func clamp(value, lower, upper int) int {
	return max(lower, min(value, upper))
}
