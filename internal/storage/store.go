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
