package analyze

import (
	"math"
	"strings"
)

func analysisQuality(pages []Page, questions []Question) *AnalysisQuality {
	pageSummary := summarizePageQuality(pages)
	quality := &AnalysisQuality{
		OCRReviewPages: pageSummary.ocrReviewPages,
		Warnings:       []string{},
	}
	quality.OCRAssessmentCoveragePercent = percent(pageSummary.ocrAssessedPages, len(pages))
	if pageSummary.ocrAssessedPages > 0 {
		quality.AverageOCRConfidence = roundTo(
			pageSummary.ocrConfidence/float64(pageSummary.ocrAssessedPages),
			2,
		)
		quality.MinimumOCRConfidence = roundTo(pageSummary.minimumOCRConfidence, 2)
	}
	quality.ClassificationCoveragePercent = percent(pageSummary.classifiedPages, len(pages))
	if pageSummary.classifiedPages > 0 {
		quality.AverageClassificationConfidence = roundTo(
			pageSummary.classificationConfidence/float64(pageSummary.classifiedPages),
			2,
		)
	}

	promptMatches := 0
	analyzedQuestions := 0
	evidenceBackedQuestions := 0
	boundaryReviewQuestions := 0
	for _, question := range questions {
		hasBoundary := strings.TrimSpace(question.Boundary) != ""
		boundaryUncertain := hasBoundary && normalizeQuestionBoundary(question.Boundary) == questionBoundaryUncertain
		boundaryLowConfidence := hasBoundary && question.BoundaryConfidence < 0.7
		statusNeedsReview := questionNeedsReview(question)
		needsBoundaryReview := statusNeedsReview || boundaryUncertain || boundaryLowConfidence
		if needsBoundaryReview {
			boundaryReviewQuestions++
		}
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
	quality.OCRUnclearPercent = roundTo(
		float64(pageSummary.unclearCount)*100/float64(max(1, pageSummary.wordCount)),
		2,
	)
	quality.OverallCoveragePercent = weightedQualityCoverage(quality)
	quality.Warnings = qualityWarnings(quality, len(pages), len(questions))
	if boundaryReviewQuestions > 0 {
		quality.Warnings = append(
			quality.Warnings,
			"Some answer boundaries are uncertain; verify their page grouping before using the analysis.",
		)
	}
	quality.RequiresReview = len(quality.Warnings) > 0
	return quality
}

type pageQualitySummary struct {
	classifiedPages          int
	classificationConfidence float64
	unclearCount             int
	wordCount                int
	ocrAssessedPages         int
	ocrConfidence            float64
	minimumOCRConfidence     float64
	ocrReviewPages           []int
}

func summarizePageQuality(pages []Page) pageQualitySummary {
	summary := pageQualitySummary{
		minimumOCRConfidence: 1,
		ocrReviewPages:       []int{},
	}
	for _, page := range pages {
		if normalizePageKind(page.Kind) != "unknown" {
			summary.classifiedPages++
			summary.classificationConfidence += clampFloat(page.KindConfidence, 0, 1)
		}
		summary.unclearCount += max(0, page.UnclearCount)
		summary.wordCount += len(strings.Fields(page.Text))

		needsOCRReview := page.OCRConfidence == nil || hasReportedOCRIssue(page.OCRIssues)
		if page.OCRConfidence != nil {
			pageConfidence := clampFloat(*page.OCRConfidence, 0, 1)
			summary.ocrAssessedPages++
			summary.ocrConfidence += pageConfidence
			summary.minimumOCRConfidence = min(summary.minimumOCRConfidence, pageConfidence)
			needsOCRReview = needsOCRReview || pageConfidence < 0.7
		}
		if needsOCRReview {
			summary.ocrReviewPages = append(summary.ocrReviewPages, page.Number)
		}
	}
	return summary
}

func hasReportedOCRIssue(issues []string) bool {
	for _, issue := range issues {
		if strings.TrimSpace(issue) != "" {
			return true
		}
	}
	return false
}

func questionNeedsReview(question Question) bool {
	status := strings.ToLower(strings.TrimSpace(question.Status))
	return status == "needs review" || status == "needs_review"
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
	ocrQuality := int(math.Round(
		float64(quality.OCRAssessmentCoveragePercent) * quality.AverageOCRConfidence,
	))
	weighted := classificationQuality*10 +
		ocrQuality*10 +
		quality.PromptMatchPercent*25 +
		quality.AnalysisCoveragePercent*30 +
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
	if pageCount > 0 && quality.OCRAssessmentCoveragePercent < 90 {
		warnings = append(warnings, "Some pages are missing a semantic OCR reliability assessment.")
	}
	if len(quality.OCRReviewPages) > 0 {
		warnings = append(
			warnings,
			"Some pages have low-confidence OCR or model-reported OCR issues; verify the listed pages against the copy image.",
		)
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
