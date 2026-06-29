package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *SQLiteStore) SaveStudyCopy(ctx context.Context, record StudyCopyRecord) error {
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO study_copies (
	id, source_path, source_hash, pdf_name, candidate_name, roll_no, email, test_code, paper, copy_date,
	page_count, question_count, unclear_count, status, render_status, ocr_status, question_status, analysis_status, report_status, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	source_path = excluded.source_path,
	source_hash = excluded.source_hash,
	pdf_name = excluded.pdf_name,
	candidate_name = excluded.candidate_name,
	roll_no = excluded.roll_no,
	email = excluded.email,
	test_code = excluded.test_code,
	paper = excluded.paper,
	copy_date = excluded.copy_date,
	page_count = excluded.page_count,
	question_count = excluded.question_count,
	unclear_count = excluded.unclear_count,
	status = excluded.status,
	render_status = excluded.render_status,
	ocr_status = excluded.ocr_status,
	question_status = excluded.question_status,
	analysis_status = excluded.analysis_status,
	report_status = excluded.report_status,
	last_error = excluded.last_error,
	updated_at = excluded.updated_at`,
		record.ID, record.SourcePath, record.SourceHash, record.PDFName, record.CandidateName,
		record.RollNo, record.Email, record.TestCode, record.Paper, record.CopyDate,
		record.PageCount, record.QuestionCount, record.UnclearCount, record.Status,
		record.RenderStatus, record.OCRStatus, record.QuestionStatus, record.AnalysisStatus,
		record.ReportStatus, record.LastError, record.CreatedAt, record.UpdatedAt)
	return err
}

func (s *SQLiteStore) GetStudyCopy(ctx context.Context, id string) (StudyCopyRecord, error) {
	var record StudyCopyRecord
	err := scanStudyCopy(s.db.QueryRowContext(ctx, `SELECT id, source_path, source_hash, pdf_name, candidate_name, roll_no, email, test_code, paper, copy_date, page_count, question_count, unclear_count, status, render_status, ocr_status, question_status, analysis_status, report_status, last_error, created_at, updated_at FROM study_copies WHERE id = ?`, id), &record)
	if errors.Is(err, sql.ErrNoRows) {
		return StudyCopyRecord{}, ErrNotFound
	}
	return record, err
}

func (s *SQLiteStore) ListStudyCopies(ctx context.Context, opts StudyCopyListOptions) ([]StudyCopyRecord, error) {
	limit := normalizedStudyLimit(opts.Limit)
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	query := strings.TrimSpace(strings.ToLower(opts.Query))
	status := strings.TrimSpace(strings.ToLower(opts.Status))
	base := `SELECT id, source_path, source_hash, pdf_name, candidate_name, roll_no, email, test_code, paper, copy_date, page_count, question_count, unclear_count, status, render_status, ocr_status, question_status, analysis_status, report_status, last_error, created_at, updated_at FROM study_copies`
	where := []string{}
	args := []any{}
	if query != "" {
		like := "%" + query + "%"
		where = append(where, `(lower(pdf_name) LIKE ? OR lower(candidate_name) LIKE ? OR lower(source_path) LIKE ? OR lower(paper) LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	if status != "" && status != "all" {
		where = append(where, `lower(status) = ?`)
		args = append(args, status)
	}
	if len(where) > 0 {
		base += " WHERE " + strings.Join(where, " AND ")
	}
	base += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	return s.queryStudyCopies(ctx, base, args...)
}

func (s *SQLiteStore) SaveStudyPage(ctx context.Context, page StudyPageRecord) error {
	now := time.Now().UTC()
	if page.CreatedAt.IsZero() {
		page.CreatedAt = now
	}
	page.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO study_pages (copy_id, page_number, name, image_path, image_url, ocr_text, raw_ocr, layout_json, status, error, unclear_count, verified, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(copy_id, page_number) DO UPDATE SET
	name = excluded.name, image_path = excluded.image_path, image_url = excluded.image_url,
	ocr_text = excluded.ocr_text, raw_ocr = excluded.raw_ocr, layout_json = excluded.layout_json,
	status = excluded.status, error = excluded.error, unclear_count = excluded.unclear_count,
	verified = excluded.verified, updated_at = excluded.updated_at`,
		page.CopyID, page.PageNumber, page.Name, page.ImagePath, page.ImageURL, page.OCRText,
		page.RawOCR, page.LayoutJSON, page.Status, page.Error, page.UnclearCount,
		boolInt(page.Verified), page.CreatedAt, page.UpdatedAt)
	return err
}

func (s *SQLiteStore) ListStudyPages(ctx context.Context, copyID string) ([]StudyPageRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT copy_id, page_number, name, image_path, image_url, ocr_text, raw_ocr, layout_json, status, error, unclear_count, verified, created_at, updated_at FROM study_pages WHERE copy_id = ? ORDER BY page_number`, copyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	pages := []StudyPageRecord{}
	for rows.Next() {
		var page StudyPageRecord
		var verified int
		if err := rows.Scan(&page.CopyID, &page.PageNumber, &page.Name, &page.ImagePath, &page.ImageURL, &page.OCRText, &page.RawOCR, &page.LayoutJSON, &page.Status, &page.Error, &page.UnclearCount, &verified, &page.CreatedAt, &page.UpdatedAt); err != nil {
			return nil, err
		}
		page.Verified = verified != 0
		pages = append(pages, page)
	}
	return pages, rows.Err()
}

func (s *SQLiteStore) SaveStudyQuestion(ctx context.Context, question StudyQuestionRecord) error {
	now := time.Now().UTC()
	if question.CreatedAt.IsZero() {
		question.CreatedAt = now
	}
	question.UpdatedAt = now
	sourcePages, _ := json.Marshal(question.SourcePages)
	_, err := s.db.ExecContext(ctx, `INSERT INTO study_questions (id, copy_id, question_no, label, prompt_text, prompt_hi, marks, word_limit, answer_text, source_pages_json, status, feedback_json, analysis_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	copy_id = excluded.copy_id, question_no = excluded.question_no, label = excluded.label,
	prompt_text = excluded.prompt_text, prompt_hi = excluded.prompt_hi, marks = excluded.marks,
	word_limit = excluded.word_limit, answer_text = excluded.answer_text,
	source_pages_json = excluded.source_pages_json, status = excluded.status,
	feedback_json = excluded.feedback_json, analysis_json = excluded.analysis_json,
	updated_at = excluded.updated_at`,
		question.ID, question.CopyID, question.QuestionNo, question.Label, question.PromptText,
		question.PromptHi, question.Marks, question.WordLimit, question.AnswerText, string(sourcePages),
		question.Status, question.FeedbackJSON, question.AnalysisJSON, question.CreatedAt, question.UpdatedAt)
	return err
}

func (s *SQLiteStore) ListStudyQuestions(ctx context.Context, copyID string) ([]StudyQuestionRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, copy_id, question_no, label, prompt_text, prompt_hi, marks, word_limit, answer_text, source_pages_json, status, feedback_json, analysis_json, created_at, updated_at FROM study_questions WHERE copy_id = ? ORDER BY question_no, id`, copyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	questions := []StudyQuestionRecord{}
	for rows.Next() {
		var question StudyQuestionRecord
		var sourcePages string
		if err := rows.Scan(&question.ID, &question.CopyID, &question.QuestionNo, &question.Label, &question.PromptText, &question.PromptHi, &question.Marks, &question.WordLimit, &question.AnswerText, &sourcePages, &question.Status, &question.FeedbackJSON, &question.AnalysisJSON, &question.CreatedAt, &question.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(sourcePages), &question.SourcePages)
		questions = append(questions, question)
	}
	return questions, rows.Err()
}

func (s *SQLiteStore) SaveStudyAnalysis(ctx context.Context, analysis StudyAnalysisRecord) error {
	now := time.Now().UTC()
	if analysis.CreatedAt.IsZero() {
		analysis.CreatedAt = now
	}
	analysis.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO study_analyses (id, copy_id, scope_type, scope_id, dimension_key, provider_id, model, result_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	copy_id = excluded.copy_id, scope_type = excluded.scope_type, scope_id = excluded.scope_id,
	dimension_key = excluded.dimension_key, provider_id = excluded.provider_id, model = excluded.model,
	result_json = excluded.result_json, updated_at = excluded.updated_at`,
		analysis.ID, analysis.CopyID, analysis.ScopeType, analysis.ScopeID, analysis.DimensionKey,
		analysis.ProviderID, analysis.Model, analysis.ResultJSON, analysis.CreatedAt, analysis.UpdatedAt)
	return err
}

func (s *SQLiteStore) ListStudyAnalyses(ctx context.Context, copyID string) ([]StudyAnalysisRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, copy_id, scope_type, scope_id, dimension_key, provider_id, model, result_json, created_at, updated_at FROM study_analyses WHERE copy_id = ? ORDER BY updated_at DESC`, copyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	analyses := []StudyAnalysisRecord{}
	for rows.Next() {
		var analysis StudyAnalysisRecord
		if err := rows.Scan(&analysis.ID, &analysis.CopyID, &analysis.ScopeType, &analysis.ScopeID, &analysis.DimensionKey, &analysis.ProviderID, &analysis.Model, &analysis.ResultJSON, &analysis.CreatedAt, &analysis.UpdatedAt); err != nil {
			return nil, err
		}
		analyses = append(analyses, analysis)
	}
	return analyses, rows.Err()
}

func (s *SQLiteStore) SaveStudyBatch(ctx context.Context, batch StudyBatchRecord) error {
	now := time.Now().UTC()
	if batch.CreatedAt.IsZero() {
		batch.CreatedAt = now
	}
	batch.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO study_batches (id, status, stage, total, completed, failed, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET status = excluded.status, stage = excluded.stage, total = excluded.total, completed = excluded.completed, failed = excluded.failed, updated_at = excluded.updated_at`,
		batch.ID, batch.Status, batch.Stage, batch.Total, batch.Completed, batch.Failed, batch.CreatedAt, batch.UpdatedAt)
	return err
}

func (s *SQLiteStore) SaveStudyBatchItem(ctx context.Context, item StudyBatchItemRecord) error {
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO study_batch_items (batch_id, copy_id, stage, status, error, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(batch_id, copy_id, stage) DO UPDATE SET status = excluded.status, error = excluded.error, updated_at = excluded.updated_at`,
		item.BatchID, item.CopyID, item.Stage, item.Status, item.Error, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s *SQLiteStore) ListStudyBatchItems(ctx context.Context, batchID string) ([]StudyBatchItemRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT batch_id, copy_id, stage, status, error, created_at, updated_at FROM study_batch_items WHERE batch_id = ? ORDER BY updated_at DESC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []StudyBatchItemRecord{}
	for rows.Next() {
		var item StudyBatchItemRecord
		if err := rows.Scan(&item.BatchID, &item.CopyID, &item.Stage, &item.Status, &item.Error, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *SQLiteStore) queryStudyCopies(ctx context.Context, query string, args ...any) ([]StudyCopyRecord, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	copies := []StudyCopyRecord{}
	for rows.Next() {
		var copy StudyCopyRecord
		if err := scanStudyCopy(rows, &copy); err != nil {
			return nil, err
		}
		copies = append(copies, copy)
	}
	return copies, rows.Err()
}

func scanStudyCopy(row rowScanner, record *StudyCopyRecord) error {
	return row.Scan(&record.ID, &record.SourcePath, &record.SourceHash, &record.PDFName, &record.CandidateName, &record.RollNo, &record.Email, &record.TestCode, &record.Paper, &record.CopyDate, &record.PageCount, &record.QuestionCount, &record.UnclearCount, &record.Status, &record.RenderStatus, &record.OCRStatus, &record.QuestionStatus, &record.AnalysisStatus, &record.ReportStatus, &record.LastError, &record.CreatedAt, &record.UpdatedAt)
}

func normalizedStudyLimit(limit int) int {
	if limit <= 0 {
		return 80
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
