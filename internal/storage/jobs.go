package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

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
		`INSERT INTO jobs (id, type, status, stage, progress, current_step, total_steps, eta_seconds, progress_mode, completed_units, total_units, unit_label, progress_started_at, progress_ends_at, input, output, error, created_at, updated_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID,
		job.Type,
		job.Status,
		job.Stage,
		job.Progress,
		job.CurrentStep,
		job.TotalSteps,
		job.ETASeconds,
		job.ProgressMode,
		job.CompletedUnits,
		job.TotalUnits,
		job.UnitLabel,
		nullableTime(job.ProgressStartedAt),
		nullableTime(job.ProgressEndsAt),
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
		`SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, progress_mode, completed_units, total_units, unit_label, progress_started_at, progress_ends_at, input, output, error, created_at, updated_at, finished_at FROM jobs WHERE id = ?`,
		id,
	)
	err := scanJob(row, &job)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return job, err
}

func (s *SQLiteStore) ListJobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, progress_mode, completed_units, total_units, unit_label, progress_started_at, progress_ends_at, input, output, error, created_at, updated_at, finished_at FROM jobs ORDER BY created_at DESC`,
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

func (s *SQLiteStore) ListJobsFiltered(ctx context.Context, opts JobListOptions) ([]Job, error) {
	limit := normalizedJobListLimit(opts.Limit)
	status := opts.Status
	if status == "" || status == "recent" {
		return s.queryJobs(ctx, `SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, progress_mode, completed_units, total_units, unit_label, progress_started_at, progress_ends_at, input, output, error, created_at, updated_at, finished_at FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if status == "all" {
		return s.queryJobs(ctx, `SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, progress_mode, completed_units, total_units, unit_label, progress_started_at, progress_ends_at, input, output, error, created_at, updated_at, finished_at FROM jobs ORDER BY created_at DESC LIMIT ?`, limit)
	}
	if status == "finished" {
		return s.queryJobs(ctx, `SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, progress_mode, completed_units, total_units, unit_label, progress_started_at, progress_ends_at, input, output, error, created_at, updated_at, finished_at FROM jobs WHERE status IN (?, ?, ?) ORDER BY created_at DESC LIMIT ?`, JobStatusCompleted, JobStatusFailed, JobStatusCancelled, limit)
	}
	return s.queryJobs(ctx, `SELECT id, type, status, stage, progress, current_step, total_steps, eta_seconds, progress_mode, completed_units, total_units, unit_label, progress_started_at, progress_ends_at, input, output, error, created_at, updated_at, finished_at FROM jobs WHERE status = ? ORDER BY created_at DESC LIMIT ?`, status, limit)
}

func (s *SQLiteStore) queryJobs(ctx context.Context, query string, args ...any) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
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

func normalizedJobListLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func (s *SQLiteStore) DeleteFinishedJobs(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM jobs WHERE status IN (?, ?, ?)`, JobStatusCompleted, JobStatusFailed, JobStatusCancelled)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLiteStore) MarkRunningJobsInterrupted(ctx context.Context, reason string) (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE jobs SET status = ?, stage = ?, error = ?, updated_at = ?, finished_at = ? WHERE status = ?`,
		JobStatusFailed,
		"interrupted",
		reason,
		now,
		now,
		JobStatusRunning,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLiteStore) UpdateJob(ctx context.Context, job Job) error {
	job.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(
		ctx,
		`UPDATE jobs SET status = ?, stage = ?, progress = ?, current_step = ?, total_steps = ?, eta_seconds = ?, progress_mode = ?, completed_units = ?, total_units = ?, unit_label = ?, progress_started_at = ?, progress_ends_at = ?, input = ?, output = ?, error = ?, updated_at = ?, finished_at = ? WHERE id = ?`,
		job.Status,
		job.Stage,
		job.Progress,
		job.CurrentStep,
		job.TotalSteps,
		job.ETASeconds,
		job.ProgressMode,
		job.CompletedUnits,
		job.TotalUnits,
		job.UnitLabel,
		nullableTime(job.ProgressStartedAt),
		nullableTime(job.ProgressEndsAt),
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

func (s *SQLiteStore) DeleteJob(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner, job *Job) error {
	var finishedAt sql.NullTime
	var progressStartedAt sql.NullTime
	var progressEndsAt sql.NullTime
	err := row.Scan(
		&job.ID,
		&job.Type,
		&job.Status,
		&job.Stage,
		&job.Progress,
		&job.CurrentStep,
		&job.TotalSteps,
		&job.ETASeconds,
		&job.ProgressMode,
		&job.CompletedUnits,
		&job.TotalUnits,
		&job.UnitLabel,
		&progressStartedAt,
		&progressEndsAt,
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
	if progressStartedAt.Valid {
		job.ProgressStartedAt = progressStartedAt.Time
	}
	if progressEndsAt.Valid {
		job.ProgressEndsAt = progressEndsAt.Time
	}
	return nil
}
