package analyze

import (
	"fmt"
	"strings"
)

const topperCopyOCRPrompt = `Transcribe this UPSC answer-copy page as compact Markdown.

Preserve:
- question/answer numbers and page order
- headings, bullets, numbering, diagrams, flowcharts, maps, underlines, boxes, arrows, margin notes, marks, ticks, and evaluator comments
- visible keywords, examples, data, quotes, case studies, committee names, article numbers, schemes, and conclusion lines

Rules:
- Do not summarize the page.
- Do not correct the student's language unless the handwriting clearly says so.
- Do not repeat any line or block.
- Do not include OCR bounding boxes or detector tags.
- For diagrams/flowcharts, write only the visible labels and arrows.
- Mark unreadable words as [unclear].
- Output Markdown only.`

func topperCopyQuestionPrompt(page Page, previous *Page) string {
	return fmt.Sprintf(`Classify OCR page %d from a UPSC/Mains answer-copy bundle and extract its useful content.

Return only the JSON object required by the response schema. Do not add prose or markdown fences.

Rules:
- page_kind describes the page's primary purpose. Use "answer" for handwritten/typed candidate answers, even when a small printed question header is also visible.
- Use "question_paper" only for a page primarily listing printed questions without candidate answers.
- Use "cover", "index", "evaluation", "blank", or "other" for non-answer pages as appropriate.
- page_kind_confidence must be between 0 and 1. Use a lower value when evidence is ambiguous.
- ocr_confidence must be between 0 and 1 and assess the reliability of the supplied OCR text, independently of page_kind. Lower it for truncation, unreadable markers, implausible word substitutions, or broken sentences; list concrete issues in ocr_issues without correcting the answer.
- For a question_paper page, return every visible numbered and lettered sub-question separately in printed_questions. Preserve directive words, marks, and word limits. Leave questions empty.
- For an answer page, split candidate writing into question-wise answer blocks in questions. Include printed_questions only when a complete printed prompt is actually visible.
- For every answer block, classify its boundary semantically using the previous-page context below:
  - new_answer: this block visibly or contextually begins a distinct answer on this page.
  - continuation: this block continues the answer from the immediately preceding page and contains no distinct new-answer start.
  - uncertain: the OCR does not provide enough evidence to decide safely.
- boundary_confidence measures only that boundary decision. Use uncertain rather than guessing.
- visible_label must reproduce the exact answer label visible in the OCR. Use an empty string when no label is visible; never manufacture a generic label.
- title must contain only a visible printed question prompt or explicit heading. Do not turn the answer's first word or sentence into a title.
- Do not summarize, rewrite, improve, or remove OCR text.
- Keep all visible facts, examples, diagrams/flowchart descriptions, marks, comments, and [unclear] markers.
- If unsure, return one block for the page with status "needs review".

Previous answer-page OCR for boundary context only (never copy it into the current answer block):
%s

Current page OCR:
%s`, page.Number, previousPageBoundaryContext(previous), page.Text)
}

func topperCopyQuestionRetryPrompt(page Page, previous *Page, reason string) string {
	return fmt.Sprintf(`Retry the semantic classification and extraction for UPSC answer-copy page %d.

%s

Return exactly one valid JSON object matching the response schema and no prose.

For a question-paper page, extract every visible numbered/lettered sub-question into printed_questions and leave questions empty.
For an answer page, preserve full answer text and copy only labels that are actually visible into visible_label.
Classify each answer block boundary as new_answer, continuation, or uncertain. Use uncertain instead of inferring a boundary from topic alone.
title must be a visible printed prompt or explicit heading, never the first word of the candidate's answer.
Assess OCR reliability semantically in ocr_confidence and ocr_issues. Do not rewrite or correct the OCR.

Previous answer-page OCR for boundary context only:
%s

Current page OCR:
%s`, page.Number, reason, previousPageBoundaryContext(previous), page.Text)
}

func previousPageBoundaryContext(previous *Page) string {
	if previous == nil {
		return "[no preceding answer page supplied]"
	}
	const maxRunes = 2400
	text := []rune(strings.TrimSpace(previous.Text))
	if len(text) > maxRunes {
		text = text[len(text)-maxRunes:]
	}
	return fmt.Sprintf("Page %d:\n%s", previous.Number, string(text))
}

func topperCopyReportPrompt(pagesMarkdown string) string {
	return `Act as a senior UPSC answer-writing mentor and evidence auditor. Analyze this topper answer copy for learning and deliberate practice.

Output Markdown with these sections:
1. Copy Profile: the dominant answer-writing strategy, consistency, standout qualities, and extraction confidence.
2. Scorecard Synthesis: compare the supplied analytical rubric across answers; identify the strongest and weakest dimensions without treating the rubric as official UPSC marks.
3. Answer-Wise Analysis: for every answer, cover question demand, demand fulfilment, structure, multidimensional coverage, argument depth, evidence/value addition, visual/presentation choices, conclusion, examiner-friendly signals, gaps, and the highest-leverage improvement. Cite the answer label and source page(s).
4. Repeated Winning Patterns: techniques that recur across multiple answers, why they work, and where they should be reused.
5. What Not to Copy Blindly: weak, risky, context-specific, or merely decorative patterns that an aspirant should not imitate without judgment.
6. Gap Map: repeated missing dimensions, unsupported claims, weak transitions, generic conclusions, presentation issues, and OCR limitations. Separate copy weaknesses from extraction uncertainty.
7. Reusable Answer-Writing Playbook: frameworks, keywords, opening and conclusion patterns, diagram/flowchart/map techniques, evidence-integration patterns, and compact presentation methods grounded in this copy.
8. Deliberate-Practice Plan: prioritized drills for the next 7 answers, with an observable success criterion for each drill.

Rules:
- Base every judgment on the extracted answer text or supplied structured analysis. Prefer short quotations or exact visible features as evidence.
- Do not invent official model answers or facts not visible in the copy.
- Do not present the analytical scorecard as predicted UPSC marks, rank, or an official examiner assessment.
- Preserve answer numbers and page references. Do not skip an answer that has usable text.
- Distinguish observation (what is visible), interpretation (why it may work), and recommendation (what to practise).
- Flag contradictions between the structured analysis and answer text instead of silently accepting them.
- Treat OCR failure markers and [unclear] text as extraction limitations, not student mistakes.
- Prioritize repeated, transferable patterns over generic praise. Keep every recommendation concrete and testable.

Extracted topper answers and their structured per-question analysis:

` + pagesMarkdown
}

func questionDimensionsPrompt(q Question) string {
	return fmt.Sprintf(`Act as a senior UPSC answer-writing evaluator. Produce an evidence-based diagnostic of this single answer.

You must return a valid JSON object matching the exact schema below.
- Do not wrap the JSON in markdown code fences (like `+"```"+`json or `+"```"+`).
- Do not include any trailing commas.
- Escape all double quotes in string values as \".
- Escape all newlines in string values as \n (do not output literal newlines in string values).

Schema:
{
  "introduction": "Evaluate the intro pattern, relevance, precision, and effectiveness.",
  "outro": "Evaluate whether the conclusion synthesizes, answers the demand, and gives a proportionate way forward or broader linkage.",
  "transition": "Evaluate logical flow, paragraph/headings sequence, linking sentences, and balance between sections.",
  "diagram": "Evaluate diagrams, flowcharts, tables, or maps for correctness signals, integration, and value per unit of space.",
  "fact": "List only visible facts, data, examples, constitutional articles, committees, judgments, reports, schemes, thinkers, case studies, or quotations.",
  "fact_usage": "Evaluate whether visible evidence supports an argument or is merely name-dropped.",
  "custom": "Other distinctive structural, thematic, or evaluator-marking observations.",
  "demand_alignment": "State the visible or inferable demand and evaluate how directly every part addresses it. Flag uncertainty if the full question is unavailable.",
  "body_structure": "Map the body architecture and evaluate balance, sequencing, headings, transitions, and space allocation.",
  "content_depth": "Evaluate explanation, causal reasoning, trade-offs, counterpoints, synthesis, and whether claims go beyond listing.",
  "multidimensionality": "List dimensions actually covered and assess balance; distinguish missing dimensions from dimensions not required by the visible demand.",
  "presentation": "Evaluate scanability, keywords, bullets, underlining/boxes, density, legibility signals in OCR, and integration of visual devices.",
  "strengths": [
    {"point": "specific strength", "evidence": "short exact quote or visible feature", "why_it_matters": "likely examiner or reader benefit"}
  ],
  "gaps": [
    {"point": "specific weakness or risk", "evidence": "short exact quote or visible omission", "why_it_matters": "likely cost to relevance, depth, or clarity"}
  ],
  "missing_dimensions": ["important demand-relevant dimension absent from the answer"],
  "examiner_signals": ["visible evaluator marks, ticks, comments, or answer features that make evaluation easy"],
  "improvements": [
    {"priority": "high|medium|low", "change": "one concrete revision or practice action", "example": "brief form-level example using only visible content; otherwise empty"}
  ],
  "reusable_techniques": ["specific transferable technique demonstrated by this answer"],
  "scorecard": {
    "demand_fulfilment": 0,
    "structure": 0,
    "content_depth": 0,
    "evidence": 0,
    "multidimensionality": 0,
    "presentation": 0,
    "conclusion": 0,
    "overall_percent": 0,
    "estimated_band": "exceptional|strong|competent|developing|insufficient evidence",
    "confidence": "high|medium|low",
    "rationale": "brief evidence-based reason for the analytical rubric result"
  }
}

Rules:
- Score each scorecard dimension from 0 to 5: 0=not observable, 1=weak, 2=developing, 3=competent, 4=strong, 5=exceptional. overall_percent must be 0-100 and consistent with those scores.
- The scorecard is a learning diagnostic, not predicted UPSC marks and not an official examiner assessment.
- Be concise but specific. Return at most 4 strengths, 4 gaps, and 4 improvements, ordered by learning value.
- Every strength and gap must contain visible evidence. Use a short exact quote where OCR permits; otherwise name the exact structural feature or omission.
- If a dimension is completely missing, state "Not present" and note whether it was actually relevant to the demand.
- Do not penalize a missing dimension unless it is relevant to the visible question demand.
- Do not invent facts, model-answer content, evaluator intent, or marks. Suggested improvements may restructure visible material but must not add external factual claims.
- Treat [unclear] and OCR failures as extraction uncertainty, lower confidence accordingly, and never label them as student errors.
- Base the analysis strictly on the question/title and answer OCR below.

Question: %s
Title: %s
Answer OCR:
%s`, q.Label, q.Title, q.AnswerMarkdown)
}
