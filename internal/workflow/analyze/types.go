package analyze

import (
	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
)

type Service struct {
	tools            config.ToolConfig
	runner           tool.Runner
	ocrProvider      provider.Provider
	questionProvider provider.Provider
	reportProvider   provider.Provider
	artifactDir      string
}

type Request struct {
	Model           string `json:"model"`
	OCRModel        string `json:"ocr_model"`
	QuestionModel   string `json:"question_model"`
	ReportModel     string `json:"report_model"`
	Path            string `json:"path"`
	DPI             int    `json:"dpi"`
	RenderWorkers   int    `json:"render_workers"`
	Workers         int    `json:"workers"`
	QuestionSplit   bool   `json:"question_split"`
	QuestionWorkers int    `json:"question_workers"`
	UnloadModels    bool   `json:"unload_models"`
}

type ReprocessRequest struct {
	Model           string `json:"model"`
	OCRModel        string `json:"ocr_model"`
	QuestionModel   string `json:"question_model"`
	ReportModel     string `json:"report_model"`
	Action          string `json:"action"`
	PageNumbers     []int  `json:"page_numbers"`
	QuestionSplit   bool   `json:"question_split"`
	QuestionWorkers int    `json:"question_workers"`
	Workers         int    `json:"workers"`
	UnloadModels    bool   `json:"unload_models"`
}

type Page struct {
	Number       int    `json:"number"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	ImageURL     string `json:"image_url"`
	Text         string `json:"text"`
	UnclearCount int    `json:"unclear_count"`
	Verified     bool   `json:"verified"`
}

type Question struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Title          string `json:"title,omitempty"`
	AnswerMarkdown string `json:"answer_markdown"`
	SourcePages    []int  `json:"source_pages"`
	Status         string `json:"status"`
}

type Response struct {
	Kind      string     `json:"kind"`
	ReviewID  string     `json:"review_id"`
	PDFName   string     `json:"pdf_name"`
	Pages     []Page     `json:"pages"`
	Questions []Question `json:"questions"`
	Report    string     `json:"report"`
}

type Option func(*Service)

type ProgressFunc func(stage string, completed int, total int, label string)

func WithArtifactDir(path string) Option {
	return func(s *Service) {
		s.artifactDir = path
	}
}

func WithQuestionProvider(p provider.Provider) Option {
	return func(s *Service) {
		if p != nil {
			s.questionProvider = p
		}
	}
}

func WithReportProvider(p provider.Provider) Option {
	return func(s *Service) {
		if p != nil {
			s.reportProvider = p
		}
	}
}
