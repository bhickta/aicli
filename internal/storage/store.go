package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Job struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	input TEXT NOT NULL DEFAULT '',
	output TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);
`)
	return err
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
		`INSERT INTO jobs (id, type, status, input, output, error, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.Type,
		job.Status,
		job.Input,
		job.Output,
		job.Error,
		job.CreatedAt,
		job.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetJob(ctx context.Context, id string) (Job, error) {
	var job Job
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, type, status, input, output, error, created_at, updated_at FROM jobs WHERE id = ?`,
		id,
	).Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&job.Input,
		&job.Output,
		&job.Error,
		&job.CreatedAt,
		&job.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return job, err
}

func (s *SQLiteStore) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, type, status, input, output, error, created_at, updated_at FROM jobs ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		var job Job
		if err := rows.Scan(
			&job.ID,
			&job.Type,
			&job.Status,
			&job.Input,
			&job.Output,
			&job.Error,
			&job.CreatedAt,
			&job.UpdatedAt,
		); err != nil {
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
		`UPDATE jobs SET status = ?, input = ?, output = ?, error = ?, updated_at = ? WHERE id = ?`,
		job.Status,
		job.Input,
		job.Output,
		job.Error,
		job.UpdatedAt,
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
