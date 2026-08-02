package analyze

import "github.com/bhickta/aicli/internal/provider"

func pageQuestionSplitJSONSchema() *provider.JSONSchema {
	printedQuestion := strictObjectSchema(
		map[string]any{
			"label":  stringSchema(),
			"prompt": stringSchema(),
		},
		"label", "prompt",
	)
	return &provider.JSONSchema{
		Name:        "topper_copy_page",
		Description: "Semantic page classification, OCR reliability, printed prompts, and candidate answer blocks.",
		Strict:      true,
		Schema: strictObjectSchema(
			map[string]any{
				"page_kind":             enumStringSchema("answer", "question_paper", "cover", "index", "evaluation", "blank", "other"),
				"page_kind_confidence":  numberSchema(0, 1),
				"classification_reason": stringSchema(),
				"ocr_confidence":        numberSchema(0, 1),
				"ocr_issues":            arraySchema(stringSchema(), 8),
				"printed_questions":     arraySchema(printedQuestion, 30),
			},
			"page_kind",
			"page_kind_confidence",
			"classification_reason",
			"ocr_confidence",
			"ocr_issues",
			"printed_questions",
		),
	}
}

func answerBoundaryLedgerJSONSchema() *provider.JSONSchema {
	decision := strictObjectSchema(
		map[string]any{
			"page_number":         integerSchema(1, 10000),
			"boundary":            enumStringSchema("new_answer", "continuation", "uncertain"),
			"boundary_confidence": numberSchema(0, 1),
			"visible_label":       stringSchema(),
			"label_evidence":      stringSchema(),
			"reason":              stringSchema(),
		},
		"page_number",
		"boundary",
		"boundary_confidence",
		"visible_label",
		"label_evidence",
		"reason",
	)
	return &provider.JSONSchema{
		Name:        "topper_answer_boundary_ledger",
		Description: "One evidence-grounded semantic answer-boundary decision per OCR page.",
		Strict:      true,
		Schema: strictObjectSchema(
			map[string]any{
				"decisions": arraySchema(decision, 500),
			},
			"decisions",
		),
	}
}

func questionDimensionsJSONSchema() *provider.JSONSchema {
	point := strictObjectSchema(
		map[string]any{
			"point":          stringSchema(),
			"evidence":       stringSchema(),
			"why_it_matters": stringSchema(),
		},
		"point", "evidence", "why_it_matters",
	)
	improvement := strictObjectSchema(
		map[string]any{
			"priority": enumStringSchema("high", "medium", "low"),
			"change":   stringSchema(),
			"example":  stringSchema(),
		},
		"priority", "change", "example",
	)
	scorecard := strictObjectSchema(
		map[string]any{
			"demand_fulfilment":   integerSchema(0, 5),
			"structure":           integerSchema(0, 5),
			"content_depth":       integerSchema(0, 5),
			"evidence":            integerSchema(0, 5),
			"multidimensionality": integerSchema(0, 5),
			"presentation":        integerSchema(0, 5),
			"conclusion":          integerSchema(0, 5),
			"overall_percent":     integerSchema(0, 100),
			"estimated_band":      enumStringSchema("exceptional", "strong", "competent", "developing", "insufficient evidence"),
			"confidence":          enumStringSchema("high", "medium", "low"),
			"rationale":           stringSchema(),
		},
		"demand_fulfilment",
		"structure",
		"content_depth",
		"evidence",
		"multidimensionality",
		"presentation",
		"conclusion",
		"overall_percent",
		"estimated_band",
		"confidence",
		"rationale",
	)
	properties := map[string]any{
		"introduction":        stringSchema(),
		"outro":               stringSchema(),
		"transition":          stringSchema(),
		"diagram":             stringSchema(),
		"fact":                stringSchema(),
		"fact_usage":          stringSchema(),
		"custom":              stringSchema(),
		"demand_alignment":    stringSchema(),
		"body_structure":      stringSchema(),
		"content_depth":       stringSchema(),
		"multidimensionality": stringSchema(),
		"presentation":        stringSchema(),
		"strengths":           arraySchema(point, maxAnalysisItems),
		"gaps":                arraySchema(point, maxAnalysisItems),
		"missing_dimensions":  arraySchema(stringSchema(), 8),
		"examiner_signals":    arraySchema(stringSchema(), 8),
		"improvements":        arraySchema(improvement, maxAnalysisItems),
		"reusable_techniques": arraySchema(stringSchema(), 8),
		"scorecard":           scorecard,
	}
	return &provider.JSONSchema{
		Name:        "topper_question_analysis",
		Description: "Evidence-based, typed diagnostic for one UPSC answer.",
		Strict:      true,
		Schema: strictObjectSchema(
			properties,
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
			"strengths",
			"gaps",
			"missing_dimensions",
			"examiner_signals",
			"improvements",
			"reusable_techniques",
			"scorecard",
		),
	}
}

func directPDFReconciliationJSONSchema(pageCount int, candidateCount int) *provider.JSONSchema {
	maxPages := max(1, pageCount)
	maxCandidates := max(1, candidateCount)
	maxGroups := max(1, pageCount+candidateCount)
	group := strictObjectSchema(
		map[string]any{
			"id":                     stringSchema(),
			"status":                 enumStringSchema(directPDFQuestionAnswered, directPDFQuestionUnanswered),
			"candidate_ids":          arraySchema(stringSchema(), maxCandidates),
			"canonical_candidate_id": stringSchema(),
			"label":                  stringSchema(),
			"title":                  stringSchema(),
			"source_pages":           arraySchema(integerSchema(1, maxPages), maxPages),
			"merged_answer_markdown": stringSchema(),
			"confidence":             numberSchema(0, 1),
			"reason":                 stringSchema(),
		},
		"id",
		"status",
		"candidate_ids",
		"canonical_candidate_id",
		"label",
		"title",
		"source_pages",
		"merged_answer_markdown",
		"confidence",
		"reason",
	)
	inventory := strictObjectSchema(
		map[string]any{
			"visible_question_slots": integerSchema(0, maxGroups),
			"answered":               integerSchema(0, maxGroups),
			"unanswered":             integerSchema(0, maxGroups),
		},
		"visible_question_slots",
		"answered",
		"unanswered",
	)
	answerAnalysis := strictObjectSchema(
		map[string]any{
			"group_id": stringSchema(),
			"analysis": stringSchema(),
		},
		"group_id",
		"analysis",
	)
	report := strictObjectSchema(
		map[string]any{
			"copy_profile":                     stringSchema(),
			"scorecard_synthesis":              stringSchema(),
			"answer_analyses":                  arraySchema(answerAnalysis, maxGroups),
			"repeated_winning_patterns":        stringSchema(),
			"what_not_to_copy_blindly":         stringSchema(),
			"gap_map":                          stringSchema(),
			"reusable_answer_writing_playbook": stringSchema(),
			"deliberate_practice_plan":         stringSchema(),
		},
		"copy_profile",
		"scorecard_synthesis",
		"answer_analyses",
		"repeated_winning_patterns",
		"what_not_to_copy_blindly",
		"gap_map",
		"reusable_answer_writing_playbook",
		"deliberate_practice_plan",
	)
	return &provider.JSONSchema{
		Name:        "topper_copy_reconciliation",
		Description: "Semantic full-PDF answer grouping, unanswered-slot inventory, and evidence-grounded report sections.",
		Strict:      true,
		Schema: strictObjectSchema(
			map[string]any{
				"groups":    arraySchema(group, maxGroups),
				"inventory": inventory,
				"warnings":  arraySchema(stringSchema(), maxGroups),
				"report":    report,
			},
			"groups",
			"inventory",
			"warnings",
			"report",
		),
	}
}

func strictObjectSchema(properties map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           properties,
		"required":             required,
		"additionalProperties": false,
	}
}

func stringSchema() map[string]any {
	return map[string]any{"type": "string"}
}

func enumStringSchema(values ...string) map[string]any {
	return map[string]any{"type": "string", "enum": values}
}

func numberSchema(minimum, maximum float64) map[string]any {
	return map[string]any{"type": "number", "minimum": minimum, "maximum": maximum}
}

func integerSchema(minimum, maximum int) map[string]any {
	return map[string]any{"type": "integer", "minimum": minimum, "maximum": maximum}
}

func arraySchema(items map[string]any, maxItems int) map[string]any {
	return map[string]any{"type": "array", "items": items, "maxItems": maxItems}
}
