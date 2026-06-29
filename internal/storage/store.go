package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Job struct {
	ID                string    `json:"id"`
	Type              string    `json:"type"`
	Status            string    `json:"status"`
	Stage             string    `json:"stage"`
	Progress          float64   `json:"progress"`
	CurrentStep       int       `json:"current_step"`
	TotalSteps        int       `json:"total_steps"`
	ETASeconds        int       `json:"eta_seconds"`
	ProgressMode      string    `json:"progress_mode"`
	CompletedUnits    int       `json:"completed_units"`
	TotalUnits        int       `json:"total_units"`
	UnitLabel         string    `json:"unit_label"`
	ProgressStartedAt time.Time `json:"progress_started_at,omitempty"`
	ProgressEndsAt    time.Time `json:"progress_ends_at,omitempty"`
	Input             string    `json:"input"`
	Output            string    `json:"output"`
	Error             string    `json:"error"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	FinishedAt        time.Time `json:"finished_at,omitempty"`
}

type Store interface {
	Migrate() error
	CreateJob(ctx context.Context, job Job) error
	GetJob(ctx context.Context, id string) (Job, error)
	ListJobs(ctx context.Context) ([]Job, error)
	UpdateJob(ctx context.Context, job Job) error
}

type JobDeleter interface {
	DeleteJob(ctx context.Context, id string) error
}

type JobListOptions struct {
	Status string
	Limit  int
}

type TopperReviewRecord struct {
	ID            string    `json:"id"`
	JobID         string    `json:"job_id"`
	PDFName       string    `json:"pdf_name"`
	SourcePath    string    `json:"source_path"`
	ProviderID    string    `json:"provider_id"`
	Model         string    `json:"model"`
	PageCount     int       `json:"page_count"`
	QuestionCount int       `json:"question_count"`
	UnclearCount  int       `json:"unclear_count"`
	Status        string    `json:"status"`
	ReviewJSON    string    `json:"review_json,omitempty"`
	SearchText    string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type TopperReviewListOptions struct {
	Query  string
	Limit  int
	Offset int
}

type StudyCopyRecord struct {
	ID             string    `json:"id"`
	SourcePath     string    `json:"source_path"`
	SourceHash     string    `json:"source_hash"`
	PDFName        string    `json:"pdf_name"`
	CandidateName  string    `json:"candidate_name"`
	RollNo         string    `json:"roll_no"`
	Email          string    `json:"email"`
	TestCode       string    `json:"test_code"`
	Paper          string    `json:"paper"`
	CopyDate       string    `json:"copy_date"`
	PageCount      int       `json:"page_count"`
	QuestionCount  int       `json:"question_count"`
	UnclearCount   int       `json:"unclear_count"`
	Status         string    `json:"status"`
	RenderStatus   string    `json:"render_status"`
	OCRStatus      string    `json:"ocr_status"`
	QuestionStatus string    `json:"question_status"`
	AnalysisStatus string    `json:"analysis_status"`
	ReportStatus   string    `json:"report_status"`
	LastError      string    `json:"last_error"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type StudyCopyListOptions struct {
	Query  string
	Status string
	Limit  int
	Offset int
}

type StudyPageRecord struct {
	CopyID       string    `json:"copy_id"`
	PageNumber   int       `json:"page_number"`
	Name         string    `json:"name"`
	ImagePath    string    `json:"image_path"`
	ImageURL     string    `json:"image_url"`
	OCRText      string    `json:"ocr_text"`
	RawOCR       string    `json:"raw_ocr"`
	LayoutJSON   string    `json:"layout_json"`
	Status       string    `json:"status"`
	Error        string    `json:"error"`
	UnclearCount int       `json:"unclear_count"`
	Verified     bool      `json:"verified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type StudyQuestionRecord struct {
	ID           string    `json:"id"`
	CopyID       string    `json:"copy_id"`
	QuestionNo   int       `json:"question_no"`
	Label        string    `json:"label"`
	PromptText   string    `json:"prompt_text"`
	PromptHi     string    `json:"prompt_hi"`
	Marks        int       `json:"marks"`
	WordLimit    int       `json:"word_limit"`
	AnswerText   string    `json:"answer_text"`
	SourcePages  []int     `json:"source_pages"`
	Status       string    `json:"status"`
	FeedbackJSON string    `json:"feedback_json"`
	AnalysisJSON string    `json:"analysis_json"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type StudyAnalysisRecord struct {
	ID           string    `json:"id"`
	CopyID       string    `json:"copy_id"`
	ScopeType    string    `json:"scope_type"`
	ScopeID      string    `json:"scope_id"`
	DimensionKey string    `json:"dimension_key"`
	ProviderID   string    `json:"provider_id"`
	Model        string    `json:"model"`
	ResultJSON   string    `json:"result_json"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type StudyBatchRecord struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	Stage     string    `json:"stage"`
	Total     int       `json:"total"`
	Completed int       `json:"completed"`
	Failed    int       `json:"failed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StudyBatchItemRecord struct {
	BatchID   string    `json:"batch_id"`
	CopyID    string    `json:"copy_id"`
	Stage     string    `json:"stage"`
	Status    string    `json:"status"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

var ErrNotFound = errors.New("not found")

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
