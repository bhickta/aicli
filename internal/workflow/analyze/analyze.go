package analyze

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/document"
)

type Service struct {
	tools    config.ToolConfig
	runner   tool.Runner
	provider provider.Provider
}

type Request struct {
	Model         string `json:"model"`
	Path          string `json:"path"`
	DPI           int    `json:"dpi"`
	RenderWorkers int    `json:"render_workers"`
	Workers       int    `json:"workers"`
}

type Page struct {
	Path string `json:"path"`
	Text string `json:"text"`
}

type Response struct {
	Pages  []Page `json:"pages"`
	Report string `json:"report"`
}

const defaultTopperCopyDPI = 300

const topperCopyOCRPrompt = `Extract this UPSC topper answer-copy page as Markdown.

Preserve:
- question/answer numbers and page order
- headings, subheadings, bullets, numbering, tables, diagrams, flowcharts, maps, underlines, boxes, arrows, margin notes, marks, ticks, and evaluator comments
- visible keywords, examples, data, quotes, case studies, committee names, article numbers, schemes, and conclusion lines

Rules:
- Do not summarize the page.
- Do not correct the student's language unless the handwriting clearly says so.
- Mark unreadable words as [unclear].
- Output Markdown only.`

func New(tools config.ToolConfig, runner tool.Runner, provider provider.Provider) *Service {
	return &Service{tools: tools, runner: runner, provider: provider}
}

func (s *Service) Run(ctx context.Context, req Request) (Response, error) {
	if strings.TrimSpace(req.Path) == "" {
		return Response{}, errors.New("path is required")
	}
	if req.DPI == 0 {
		req.DPI = defaultTopperCopyDPI
	}
	images, cleanup, err := document.RenderPDFToImages(ctx, s.tools, s.runner, req.Path, req.DPI, req.RenderWorkers)
	if err != nil {
		return Response{}, err
	}
	defer cleanup()

	inputs := make([]document.ImageInput, 0, len(images))
	for i, imagePath := range images {
		inputs = append(inputs, document.ImageInput{
			Name:     "page-" + strconv.Itoa(i+1),
			Path:     imagePath,
			MIMEType: "image/jpeg",
		})
	}
	ocrPages, err := document.OCRImages(
		ctx,
		s.provider,
		req.Model,
		inputs,
		topperCopyOCRPrompt,
		req.Workers,
	)
	if err != nil {
		return Response{}, err
	}
	pages := make([]Page, 0, len(ocrPages))
	for _, page := range ocrPages {
		pages = append(pages, Page{Path: page.Path, Text: page.Text})
	}

	report, err := s.report(ctx, req.Model, pages)
	if err != nil {
		return Response{}, err
	}
	return Response{Pages: pages, Report: report}, nil
}

func (s *Service) report(ctx context.Context, model string, pages []Page) (string, error) {
	var combined strings.Builder
	for i, page := range pages {
		combined.WriteString("## Page ")
		combined.WriteString(strconv.Itoa(i + 1))
		combined.WriteString("\n")
		combined.WriteString(page.Text)
		combined.WriteString("\n\n")
	}
	res, err := s.provider.Chat(ctx, provider.ChatRequest{
		Model: model,
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: topperCopyReportPrompt(combined.String()),
			},
		},
		Temperature: 0,
		MaxTokens:   4000,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res.Content), nil
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
