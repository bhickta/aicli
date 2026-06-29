package analyze

import "fmt"

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

func topperCopyQuestionPrompt(page Page) string {
	return fmt.Sprintf(`Split this OCR from UPSC topper answer-copy page %d into question-wise answer blocks.

You must return a valid JSON object matching the exact schema below.
- Do not wrap the JSON in markdown code fences (like ` + "```" + `json or ` + "```" + `).
- Do not include any trailing commas.
- Do not include any comments or additional text.
- Escape all double quotes in string values as \".
- Escape all newlines in string values as \n (do not output literal newlines in string values).

Schema:
{
  "questions": [
    {
      "label": "Q1",
      "title": "optional question heading if visible",
      "answer_markdown": "complete OCR text for only this answer block",
      "status": "detected"
    }
  ]
}

Rules:
- Do not summarize, rewrite, improve, or remove OCR text.
- Keep all visible facts, examples, diagrams/flowchart descriptions, marks, comments, and [unclear] markers.
- If the page has continuation of a previous answer, use the same visible question label if present; otherwise use "Page %d continuation".
- If unsure, return one block for the page with status "needs review".

OCR:
%s`, page.Number, page.Number, page.Text)
}

func topperCopyReportPrompt(pagesMarkdown string) string {
	return `Analyze this UPSC topper answer copy for learning and answer-writing improvement.

Output Markdown with these sections:
1. Executive Summary: 5-8 high-yield lessons from the copy.
2. Answer-Wise Analysis: for each answer, identify demand of question, structure used, dimensions covered, intro/conclusion pattern, examples/data/value-addition, diagrams/flowcharts/maps, presentation choices, and likely scoring cues.
3. Reusable Patterns: frameworks, keywords, opening lines, conclusion styles, diagrams, examples, and enrichment techniques that can be reused.
4. Weak Spots or Risks: missing dimensions, overlong parts, vague claims, weak presentation, or OCR-unclear areas.
5. Action Checklist: concrete habits to copy in future answers.

Rules:
- Base every point only on the extracted pages below.
- Do not invent official model answers or facts not visible in the copy.
- Preserve answer numbers and page references when possible.
- Treat OCR failure markers and [unclear] text as extraction limitations, not student mistakes.
- Keep the report concise but specific.

Extracted topper copy pages:

` + pagesMarkdown
}

func questionDimensionsPrompt(q Question) string {
	return fmt.Sprintf(`Analyze the structural dimensions of this single UPSC answer.

You must return a valid JSON object matching the exact schema below.
- Do not wrap the JSON in markdown code fences (like ` + "```" + `json or ` + "```" + `).
- Do not include any trailing commas.
- Escape all double quotes in string values as \".
- Escape all newlines in string values as \n (do not output literal newlines in string values).

Schema:
{
  "introduction": "Evaluate the quality of the intro. What category is it (e.g. definition, data, quote)? Why does it work or not work?",
  "outro": "Evaluate the conclusion/outro. Does it summarize, provide a way forward, or link to a broader theme? Is it effective?",
  "transition": "Evaluate the transitions between paragraphs and headings. Is the flow logical? Do they use linking sentences?",
  "diagram": "Evaluate any diagrams, flowcharts, or maps. Do they add value or just consume space? Are they well integrated?"
}

Rules:
- Be concise but highly specific to the provided text.
- If a dimension is completely missing (e.g., no diagram), state "Not present" and briefly note if it was a missed opportunity.
- Base your analysis strictly on the provided answer text.

Question: %s
Title: %s
Answer OCR:
%s`, q.Label, q.Title, q.AnswerMarkdown)
}
