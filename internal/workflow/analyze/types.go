package analyze

import (
	"log/slog"

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
	logger           *slog.Logger
	ocrCheckpoint    func(Response) error
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
	OCRBatchSize    int    `json:"ocr_batch_size"`
	OCRInputMode    string `json:"ocr_input_mode"`
	QuestionSplit   bool   `json:"question_split"`
	QuestionWorkers int    `json:"question_workers"`
	UnloadModels    bool   `json:"unload_models"`
	ForceOCR        bool   `json:"force_ocr"`
	ReviewID        string `json:"-"`
	OCRPages        []Page `json:"-"`
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
	OCRBatchSize    int    `json:"ocr_batch_size"`
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

type QuestionDimensions struct {
	Introduction string `json:"introduction"`
	Outro        string `json:"outro"`
	Transition   string `json:"transition"`
	Diagram      string `json:"diagram"`
}

type Question struct {
	ID             string              `json:"id"`
	Label          string              `json:"label"`
	Title          string              `json:"title,omitempty"`
	AnswerMarkdown string              `json:"answer_markdown"`
	SourcePages    []int               `json:"source_pages"`
	Status         string              `json:"status"`
	Dimensions     *QuestionDimensions `json:"dimensions,omitempty"`
}

type Response struct {
	Kind       string     `json:"kind"`
	ReviewID   string     `json:"review_id"`
	PDFName    string     `json:"pdf_name"`
	SourceMode string     `json:"source_mode,omitempty"`
	Pages      []Page     `json:"pages"`
	Questions  []Question `json:"questions"`
	Report     string     `json:"report"`
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

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

func WithOCRCheckpoint(save func(Response) error) Option {
	return func(s *Service) {
		s.ocrCheckpoint = save
	}
}
