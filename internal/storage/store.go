package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type Job struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	Stage       string    `json:"stage"`
	Progress    float64   `json:"progress"`
	CurrentStep int       `json:"current_step"`
	TotalSteps  int       `json:"total_steps"`
	ETASeconds  int       `json:"eta_seconds"`
	Input       string    `json:"input"`
	Output      string    `json:"output"`
	Error       string    `json:"error"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	FinishedAt  time.Time `json:"finished_at,omitempty"`
}

type Store interface {
	Migrate() error
	CreateJob(ctx context.Context, job Job) error
	GetJob(ctx context.Context, id string) (Job, error)
	ListJobs(ctx context.Context) ([]Job, error)
	UpdateJob(ctx context.Context, job Job) error
}

var ErrNotFound = errors.New("not found")

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS jobs (
	id TEXT PRIMARY KEY,
	type TEXT NOT NULL,
	status TEXT NOT NULL,
	stage TEXT NOT NULL DEFAULT '',
	progress REAL NOT NULL DEFAULT 0,
	current_step INTEGER NOT NULL DEFAULT 0,
	total_steps INTEGER NOT NULL DEFAULT 0,
	eta_seconds INTEGER NOT NULL DEFAULT 0,
	input TEXT NOT NULL DEFAULT '',
	output TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	finished_at TIMESTAMP
);
`)
	if err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE jobs ADD COLUMN stage TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE jobs ADD COLUMN progress REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE jobs ADD COLUMN current_step INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE jobs ADD COLUMN total_steps INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE jobs ADD COLUMN eta_seconds INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE jobs ADD COLUMN finished_at TIMESTAMP`,
	} {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) CreateJob(ctx context.Context, job Job) error {
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = job.CreatedAt
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO jobs (id, type, status, stage, progress, current_step, total_steps, eta_seconds, input, output, error, created_at, updated_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.Type,
		job.Status,
		job.Stage,
		job.Progress,
		job.CurrentStep,
		job.TotalSteps,
		job.ETASeconds,
		job.Input,
		job.Output,
		job.Error,
		job.CreatedAt,
		job.UpdatedAt,
		nullableTime(job.FinishedAt),
	)
	return err
}

func (s *SQLiteStore) GetJob(ctx context.Context, id string) (Job, error) {
	var job Job
	row := s.db.QueryRowContext(
		ctx,
		`SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, input, output, error, created_at, updated_at, finished_at FROM jobs WHERE id = ?`,
		id,
	)
	err := scanJob(row, &job)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return job, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner, job *Job) error {
	var finishedAt sql.NullTime
	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&job.Stage,
		&job.Progress,
		&job.CurrentStep,
		&job.TotalSteps,
		&job.ETASeconds,
		&job.Input,
		&job.Output,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
		&finishedAt,
	)
	if err != nil {
		return err
	}
	if finishedAt.Valid {
		job.FinishedAt = finishedAt.Time
	}
	return nil
}

func (s *SQLiteStore) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, input, output, error, created_at, updated_at, finished_at FROM jobs ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		var job Job
		if err := scanJob(rows, &job); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (s *SQLiteStore) UpdateJob(ctx context.Context, job Job) error {
	job.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE jobs SET status = ?, stage = ?, progress = ?, current_step = ?, total_steps = ?, eta_seconds = ?, input = ?, output = ?, error = ?, updated_at = ?, finished_at = ? WHERE id = ?`,
		job.Status,
		job.Stage,
		job.Progress,
		job.CurrentStep,
		job.TotalSteps,
		job.ETASeconds,
		job.Input,
		job.Output,
		job.Error,
		job.UpdatedAt,
		nullableTime(job.FinishedAt),
		job.ID,
	)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}
