package analyze

import (
	"fmt"
	"strconv"
	"strings"
)

func buildDirectPDFReconciliationReport(questions []Question, plan directPDFReconciliation) (string, error) {
	if err := validateDirectPDFReconciliationInventory(plan.Inventory, questions); err != nil {
		return "", err
	}
	sectionValues := []struct {
		name  string
		value string
	}{
		{name: "copy_profile", value: plan.Report.CopyProfile},
		{name: "scorecard_synthesis", value: plan.Report.ScorecardSynthesis},
		{name: "repeated_winning_patterns", value: plan.Report.RepeatedWinningPatterns},
		{name: "what_not_to_copy_blindly", value: plan.Report.WhatNotToCopyBlindly},
		{name: "gap_map", value: plan.Report.GapMap},
		{name: "reusable_answer_writing_playbook", value: plan.Report.ReusableAnswerWritingPlaybook},
		{name: "deliberate_practice_plan", value: plan.Report.DeliberatePracticePlan},
	}
	for _, section := range sectionValues {
		if strings.TrimSpace(section.value) == "" {
			return "", fmt.Errorf("direct PDF reconciliation report section %q is empty", section.name)
		}
	}

	answered := make(map[string]Question, len(questions))
	for _, question := range questions {
		if question.Status == directPDFQuestionAnswered {
			answered[question.ID] = question
		}
	}
	answerNotes := make(map[string]string, len(plan.Report.AnswerAnalyses))
	for index, note := range plan.Report.AnswerAnalyses {
		id := strings.TrimSpace(note.GroupID)
		analysis := strings.TrimSpace(note.Analysis)
		if id == "" || analysis == "" {
			return "", fmt.Errorf("direct PDF reconciliation answer analysis %d is incomplete", index+1)
		}
		if _, found := answered[id]; !found {
			return "", fmt.Errorf("direct PDF reconciliation answer analysis references non-answered group %q", id)
		}
		if _, duplicate := answerNotes[id]; duplicate {
			return "", fmt.Errorf("direct PDF reconciliation answer analysis duplicates group %q", id)
		}
		answerNotes[id] = analysis
	}
	for id := range answered {
		if _, found := answerNotes[id]; !found {
			return "", fmt.Errorf("direct PDF reconciliation report omitted answered group %q", id)
		}
	}

	var report strings.Builder
	report.WriteString("# Copy Inventory\n\n")
	fmt.Fprintf(&report, "- Visible question slots: %d\n", len(questions))
	fmt.Fprintf(&report, "- Answered: %d\n", len(answered))
	fmt.Fprintf(&report, "- Unanswered: %d\n", len(questions)-len(answered))
	if len(answered) < len(questions) {
		report.WriteString("- Unanswered slots:\n")
		for _, question := range questions {
			if question.Status != directPDFQuestionUnanswered {
				continue
			}
			fmt.Fprintf(
				&report,
				"  - %s (pages %s)\n",
				directPDFReportSingleLine(firstNonBlank(question.Label, question.ID)),
				formatDirectPDFReportPages(question.SourcePages),
			)
		}
	}

	writeDirectPDFReportSection(&report, "Copy Profile", plan.Report.CopyProfile)
	writeDirectPDFReportSection(&report, "Scorecard Synthesis", plan.Report.ScorecardSynthesis)
	report.WriteString("\n## Answer-Wise Analysis\n")
	for _, question := range questions {
		fmt.Fprintf(
			&report,
			"\n### %s (pages %s)\n\n",
			directPDFReportSingleLine(firstNonBlank(question.Label, question.ID)),
			formatDirectPDFReportPages(question.SourcePages),
		)
		if title := directPDFReportSingleLine(question.Title); title != "" {
			fmt.Fprintf(&report, "**Prompt:** %s\n\n", title)
		}
		if question.Status == directPDFQuestionUnanswered {
			report.WriteString("**Status:** Unanswered — no candidate answer content is visible on the reconciled source pages.\n")
			continue
		}
		report.WriteString(answerNotes[question.ID])
		report.WriteByte('\n')
	}
	writeDirectPDFReportSection(&report, "Repeated Winning Patterns", plan.Report.RepeatedWinningPatterns)
	writeDirectPDFReportSection(&report, "What Not to Copy Blindly", plan.Report.WhatNotToCopyBlindly)
	writeDirectPDFReportSection(&report, "Gap Map", plan.Report.GapMap)
	writeDirectPDFReportSection(&report, "Reusable Answer-Writing Playbook", plan.Report.ReusableAnswerWritingPlaybook)
	writeDirectPDFReportSection(&report, "Deliberate-Practice Plan", plan.Report.DeliberatePracticePlan)
	return strings.TrimSpace(report.String()), nil
}

func writeDirectPDFReportSection(report *strings.Builder, heading string, body string) {
	fmt.Fprintf(report, "\n## %s\n\n%s\n", heading, strings.TrimSpace(body))
}

func formatDirectPDFReportPages(pages []int) string {
	values := make([]string, len(pages))
	for index, page := range pages {
		values[index] = strconv.Itoa(page)
	}
	return strings.Join(values, ", ")
}

func directPDFReportSingleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
