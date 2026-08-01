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
	BoundaryModel   string `json:"boundary_model"`
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
	ReviewID        string `json:"review_id"`
	OCRPages        []Page `json:"-"`
}

type ReprocessRequest struct {
	Model           string `json:"model"`
	OCRModel        string `json:"ocr_model"`
	QuestionModel   string `json:"question_model"`
	BoundaryModel   string `json:"boundary_model"`
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
	Number               int      `json:"number"`
	Name                 string   `json:"name"`
	Path                 string   `json:"path"`
	ImageURL             string   `json:"image_url"`
	Text                 string   `json:"text"`
	UnclearCount         int      `json:"unclear_count"`
	Verified             bool     `json:"verified"`
	Kind                 string   `json:"kind,omitempty"`
	KindConfidence       float64  `json:"kind_confidence,omitempty"`
	ClassificationReason string   `json:"classification_reason,omitempty"`
	OCRConfidence        *float64 `json:"ocr_confidence,omitempty"`
	OCRIssues            []string `json:"ocr_issues,omitempty"`
}

type QuestionDimensions struct {
	Introduction        string                `json:"introduction"`
	Outro               string                `json:"outro"`
	Transition          string                `json:"transition"`
	Diagram             string                `json:"diagram"`
	Fact                string                `json:"fact"`
	FactUsage           string                `json:"fact_usage"`
	Custom              string                `json:"custom"`
	DemandAlignment     string                `json:"demand_alignment,omitempty"`
	BodyStructure       string                `json:"body_structure,omitempty"`
	ContentDepth        string                `json:"content_depth,omitempty"`
	Multidimensionality string                `json:"multidimensionality,omitempty"`
	Presentation        string                `json:"presentation,omitempty"`
	Strengths           []AnalysisPoint       `json:"strengths,omitempty"`
	Gaps                []AnalysisPoint       `json:"gaps,omitempty"`
	MissingDimensions   []string              `json:"missing_dimensions,omitempty"`
	ExaminerSignals     []string              `json:"examiner_signals,omitempty"`
	Improvements        []AnalysisImprovement `json:"improvements,omitempty"`
	ReusableTechniques  []string              `json:"reusable_techniques,omitempty"`
	Scorecard           *QuestionScorecard    `json:"scorecard,omitempty"`
}

type AnalysisPoint struct {
	Point        string `json:"point"`
	Evidence     string `json:"evidence,omitempty"`
	WhyItMatters string `json:"why_it_matters,omitempty"`
}

type AnalysisImprovement struct {
	Priority string `json:"priority"`
	Change   string `json:"change"`
	Example  string `json:"example,omitempty"`
}

type QuestionScorecard struct {
	DemandFulfilment    int    `json:"demand_fulfilment"`
	Structure           int    `json:"structure"`
	ContentDepth        int    `json:"content_depth"`
	Evidence            int    `json:"evidence"`
	Multidimensionality int    `json:"multidimensionality"`
	Presentation        int    `json:"presentation"`
	Conclusion          int    `json:"conclusion"`
	OverallPercent      int    `json:"overall_percent"`
	EstimatedBand       string `json:"estimated_band"`
	Confidence          string `json:"confidence"`
	Rationale           string `json:"rationale"`
}

type AnalysisQuality struct {
	ClassificationCoveragePercent   int      `json:"classification_coverage_percent"`
	AverageClassificationConfidence float64  `json:"average_classification_confidence"`
	OCRAssessmentCoveragePercent    int      `json:"ocr_assessment_coverage_percent"`
	AverageOCRConfidence            float64  `json:"average_ocr_confidence"`
	MinimumOCRConfidence            float64  `json:"minimum_ocr_confidence"`
	OCRReviewPages                  []int    `json:"ocr_review_pages"`
	PromptMatchPercent              int      `json:"prompt_match_percent"`
	AnalysisCoveragePercent         int      `json:"analysis_coverage_percent"`
	EvidenceCoveragePercent         int      `json:"evidence_coverage_percent"`
	OCRUnclearPercent               float64  `json:"ocr_unclear_percent"`
	OverallCoveragePercent          int      `json:"overall_coverage_percent"`
	RequiresReview                  bool     `json:"requires_review"`
	Warnings                        []string `json:"warnings"`
}

type CopyMetadata struct {
	SuggestedPDFName  string   `json:"suggested_pdf_name,omitempty"`
	TopperName        string   `json:"topper_name,omitempty"`
	CandidateName     string   `json:"candidate_name,omitempty"`
	Rank              string   `json:"rank,omitempty"`
	Exam              string   `json:"exam,omitempty"`
	Year              string   `json:"year,omitempty"`
	Paper             string   `json:"paper,omitempty"`
	Subject           string   `json:"subject,omitempty"`
	TestSeries        string   `json:"test_series,omitempty"`
	CoachingInstitute string   `json:"coaching_institute,omitempty"`
	TestCode          string   `json:"test_code,omitempty"`
	TestDate          string   `json:"test_date,omitempty"`
	Language          string   `json:"language,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	SearchHints       []string `json:"search_hints,omitempty"`
	Notes             string   `json:"notes,omitempty"`
}

type QuestionMetadata struct {
	Subject      string   `json:"subject,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	Subtopic     string   `json:"subtopic,omitempty"`
	SyllabusArea string   `json:"syllabus_area,omitempty"`
	Paper        string   `json:"paper,omitempty"`
	QuestionType string   `json:"question_type,omitempty"`
	Demand       string   `json:"demand,omitempty"`
	Difficulty   string   `json:"difficulty,omitempty"`
	Marks        int      `json:"marks,omitempty"`
	WordLimit    int      `json:"word_limit,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	SearchHints  []string `json:"search_hints,omitempty"`
}

type Question struct {
	ID                 string              `json:"id"`
	Label              string              `json:"label"`
	Title              string              `json:"title,omitempty"`
	AnswerMarkdown     string              `json:"answer_markdown"`
	SourcePages        []int               `json:"source_pages"`
	Status             string              `json:"status"`
	Boundary           string              `json:"boundary,omitempty"`
	BoundaryConfidence float64             `json:"boundary_confidence,omitempty"`
	Dimensions         *QuestionDimensions `json:"dimensions,omitempty"`
	Metadata           *QuestionMetadata   `json:"metadata,omitempty"`
}

type Response struct {
	Kind       string               `json:"kind"`
	ReviewID   string               `json:"review_id"`
	PDFName    string               `json:"pdf_name"`
	SourceMode string               `json:"source_mode,omitempty"`
	APICalls   int                  `json:"api_calls,omitempty"`
	Usage      *provider.TokenUsage `json:"usage,omitempty"`
	Metadata   *CopyMetadata        `json:"metadata,omitempty"`
	Pages      []Page               `json:"pages"`
	Questions  []Question           `json:"questions"`
	Report     string               `json:"report"`
	Quality    *AnalysisQuality     `json:"quality,omitempty"`
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
