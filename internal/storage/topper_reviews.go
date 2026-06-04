package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

func (s *SQLiteStore) SaveTopperReview(ctx context.Context, record TopperReviewRecord) error {
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO topper_reviews (id, job_id, pdf_name, source_path, provider_id, model, page_count, question_count, unclear_count, status, review_json, search_text, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	job_id = excluded.job_id,
	pdf_name = excluded.pdf_name,
	source_path = excluded.source_path,
	provider_id = excluded.provider_id,
	model = excluded.model,
	page_count = excluded.page_count,
	question_count = excluded.question_count,
	unclear_count = excluded.unclear_count,
	status = excluded.status,
	review_json = excluded.review_json,
	search_text = excluded.search_text,
	updated_at = excluded.updated_at`,
		record.ID,
		record.JobID,
		record.PDFName,
		record.SourcePath,
		record.ProviderID,
		record.Model,
		record.PageCount,
		record.QuestionCount,
		record.UnclearCount,
		record.Status,
		record.ReviewJSON,
		record.SearchText,
		record.CreatedAt,
		record.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) GetTopperReview(ctx context.Context, id string) (TopperReviewRecord, error) {
	var record TopperReviewRecord
	err := scanTopperReview(s.db.QueryRowContext(
		ctx,
		`SELECT id, job_id, pdf_name, source_path, provider_id, model, page_count, question_count, unclear_count, status, review_json, search_text, created_at, updated_at
FROM topper_reviews WHERE id = ?`,
		id,
	), &record)
	if errors.Is(err, sql.ErrNoRows) {
		return TopperReviewRecord{}, ErrNotFound
	}
	return record, err
}

func (s *SQLiteStore) ListTopperReviews(ctx context.Context, opts TopperReviewListOptions) ([]TopperReviewRecord, error) {
	limit := normalizedTopperReviewLimit(opts.Limit)
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	query := strings.TrimSpace(strings.ToLower(opts.Query))
	if query == "" {
		return s.queryTopperReviews(ctx, `SELECT id, job_id, pdf_name, source_path, provider_id, model, page_count, question_count, unclear_count, status, '' AS review_json, search_text, created_at, updated_at FROM topper_reviews ORDER BY updated_at DESC LIMIT ? OFFSET ?`, limit, offset)
	}
	like := "%" + query + "%"
	return s.queryTopperReviews(ctx, `SELECT id, job_id, pdf_name, source_path, provider_id, model, page_count, question_count, unclear_count, status, '' AS review_json, search_text, created_at, updated_at FROM topper_reviews WHERE lower(pdf_name) LIKE ? OR lower(source_path) LIKE ? OR lower(search_text) LIKE ? ORDER BY updated_at DESC LIMIT ? OFFSET ?`, like, like, like, limit, offset)
}

func (s *SQLiteStore) queryTopperReviews(ctx context.Context, query string, args ...any) ([]TopperReviewRecord, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []TopperReviewRecord{}
	for rows.Next() {
		var record TopperReviewRecord
		if err := scanTopperReview(rows, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func normalizedTopperReviewLimit(limit int) int {
	if limit <= 0 {
		return 30
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func scanTopperReview(row rowScanner, record *TopperReviewRecord) error {
	return row.Scan(
		&record.ID,
		&record.JobID,
		&record.PDFName,
		&record.SourcePath,
		&record.ProviderID,
		&record.Model,
		&record.PageCount,
		&record.QuestionCount,
		&record.UnclearCount,
		&record.Status,
		&record.ReviewJSON,
		&record.SearchText,
		&record.CreatedAt,
		&record.UpdatedAt,
	)
}
