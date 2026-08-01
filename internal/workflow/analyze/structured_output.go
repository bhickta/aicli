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
	question := strictObjectSchema(
		map[string]any{
			"label":           stringSchema(),
			"title":           stringSchema(),
			"answer_markdown": stringSchema(),
			"status":          enumStringSchema("detected", "needs review"),
		},
		"label", "title", "answer_markdown", "status",
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
				"questions":             arraySchema(question, 8),
			},
			"page_kind",
			"page_kind_confidence",
			"classification_reason",
			"ocr_confidence",
			"ocr_issues",
			"printed_questions",
			"questions",
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
