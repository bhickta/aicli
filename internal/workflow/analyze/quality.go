package analyze

import (
	"math"
	"strings"
)

func analysisQuality(pages []Page, questions []Question) *AnalysisQuality {
	quality := &AnalysisQuality{Warnings: []string{}}
	classifiedPages := 0
	classificationConfidence := 0.0
	unclearCount := 0
	wordCount := 0
	for _, page := range pages {
		kind := normalizePageKind(page.Kind)
		if kind != "unknown" {
			classifiedPages++
			classificationConfidence += clampFloat(page.KindConfidence, 0, 1)
		}
		unclearCount += max(0, page.UnclearCount)
		wordCount += len(strings.Fields(page.Text))
	}
	quality.ClassificationCoveragePercent = percent(classifiedPages, len(pages))
	if classifiedPages > 0 {
		quality.AverageClassificationConfidence = roundTo(
			classificationConfidence/float64(classifiedPages),
			2,
		)
	}

	promptMatches := 0
	analyzedQuestions := 0
	evidenceBackedQuestions := 0
	for _, question := range questions {
		if strings.TrimSpace(question.Title) != "" {
			promptMatches++
		}
		if question.Dimensions == nil {
			continue
		}
		analyzedQuestions++
		if hasEvidenceBackedAnalysis(question.Dimensions) {
			evidenceBackedQuestions++
		}
	}
	quality.PromptMatchPercent = percent(promptMatches, len(questions))
	quality.AnalysisCoveragePercent = percent(analyzedQuestions, len(questions))
	quality.EvidenceCoveragePercent = percent(evidenceBackedQuestions, len(questions))
	quality.OCRUnclearPercent = roundTo(float64(unclearCount)*100/float64(max(1, wordCount)), 2)
	quality.OverallCoveragePercent = weightedQualityCoverage(quality)
	quality.Warnings = qualityWarnings(quality, len(pages), len(questions))
	quality.RequiresReview = len(quality.Warnings) > 0
	return quality
}

func hasEvidenceBackedAnalysis(dimensions *QuestionDimensions) bool {
	points := append([]AnalysisPoint{}, dimensions.Strengths...)
	points = append(points, dimensions.Gaps...)
	if len(points) == 0 {
		return false
	}
	withEvidence := 0
	for _, point := range points {
		if strings.TrimSpace(point.Evidence) != "" {
			withEvidence++
		}
	}
	return withEvidence*100 >= len(points)*75
}

func weightedQualityCoverage(quality *AnalysisQuality) int {
	classificationQuality := int(math.Round(
		float64(quality.ClassificationCoveragePercent) * quality.AverageClassificationConfidence,
	))
	weighted := classificationQuality*15 +
		quality.PromptMatchPercent*25 +
		quality.AnalysisCoveragePercent*35 +
		quality.EvidenceCoveragePercent*25
	return clamp(int(math.Round(float64(weighted)/100)), 0, 100)
}

func qualityWarnings(quality *AnalysisQuality, pageCount, questionCount int) []string {
	warnings := []string{}
	if pageCount == 0 {
		warnings = append(warnings, "No OCR pages were produced.")
	} else if quality.ClassificationCoveragePercent < 90 {
		warnings = append(warnings, "Some pages could not be classified confidently.")
	} else if quality.AverageClassificationConfidence < 0.7 {
		warnings = append(warnings, "Page classifications have low model confidence and should be reviewed.")
	}
	if questionCount == 0 {
		warnings = append(warnings, "No answer blocks were detected.")
		return warnings
	}
	if quality.PromptMatchPercent < 80 {
		warnings = append(warnings, "Some answers could not be matched to an exact printed question prompt.")
	}
	if quality.AnalysisCoveragePercent < 90 {
		warnings = append(warnings, "Some answers are missing structured analysis.")
	}
	if quality.EvidenceCoveragePercent < 80 {
		warnings = append(warnings, "Some analyses lack enough answer-grounded evidence.")
	}
	if quality.OCRUnclearPercent > 2 {
		warnings = append(warnings, "OCR uncertainty is high; verify the affected pages.")
	}
	return warnings
}

func percent(part, total int) int {
	if total <= 0 {
		return 0
	}
	return clamp(int(math.Round(float64(part)*100/float64(total))), 0, 100)
}

func roundTo(value float64, places int) float64 {
	factor := math.Pow10(places)
	return math.Round(value*factor) / factor
}
