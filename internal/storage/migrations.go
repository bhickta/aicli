package storage

import "strings"

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
	progress_mode TEXT NOT NULL DEFAULT '',
	completed_units INTEGER NOT NULL DEFAULT 0,
	total_units INTEGER NOT NULL DEFAULT 0,
	unit_label TEXT NOT NULL DEFAULT '',
	progress_started_at TIMESTAMP,
	progress_ends_at TIMESTAMP,
	input TEXT NOT NULL DEFAULT '',
	output TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	finished_at TIMESTAMP
);
CREATE TABLE IF NOT EXISTS topper_reviews (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL DEFAULT '',
	pdf_name TEXT NOT NULL DEFAULT '',
	source_path TEXT NOT NULL DEFAULT '',
	provider_id TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	page_count INTEGER NOT NULL DEFAULT 0,
	question_count INTEGER NOT NULL DEFAULT 0,
	unclear_count INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT '',
	review_json TEXT NOT NULL DEFAULT '',
	search_text TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_topper_reviews_updated_at ON topper_reviews(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_topper_reviews_pdf_name ON topper_reviews(pdf_name);
CREATE TABLE IF NOT EXISTS study_copies (
	id TEXT PRIMARY KEY,
	source_path TEXT NOT NULL DEFAULT '',
	source_hash TEXT NOT NULL DEFAULT '',
	pdf_name TEXT NOT NULL DEFAULT '',
	candidate_name TEXT NOT NULL DEFAULT '',
	roll_no TEXT NOT NULL DEFAULT '',
	email TEXT NOT NULL DEFAULT '',
	test_code TEXT NOT NULL DEFAULT '',
	paper TEXT NOT NULL DEFAULT '',
	copy_date TEXT NOT NULL DEFAULT '',
	page_count INTEGER NOT NULL DEFAULT 0,
	question_count INTEGER NOT NULL DEFAULT 0,
	unclear_count INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT '',
	render_status TEXT NOT NULL DEFAULT '',
	ocr_status TEXT NOT NULL DEFAULT '',
	question_status TEXT NOT NULL DEFAULT '',
	analysis_status TEXT NOT NULL DEFAULT '',
	report_status TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_study_copies_updated_at ON study_copies(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_study_copies_pdf_name ON study_copies(pdf_name);
CREATE INDEX IF NOT EXISTS idx_study_copies_candidate_name ON study_copies(candidate_name);
CREATE INDEX IF NOT EXISTS idx_study_copies_status ON study_copies(status);
CREATE TABLE IF NOT EXISTS study_pages (
	copy_id TEXT NOT NULL,
	page_number INTEGER NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	image_path TEXT NOT NULL DEFAULT '',
	image_url TEXT NOT NULL DEFAULT '',
	ocr_text TEXT NOT NULL DEFAULT '',
	raw_ocr TEXT NOT NULL DEFAULT '',
	layout_json TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	unclear_count INTEGER NOT NULL DEFAULT 0,
	verified INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	PRIMARY KEY(copy_id, page_number)
);
CREATE INDEX IF NOT EXISTS idx_study_pages_copy ON study_pages(copy_id, page_number);
CREATE TABLE IF NOT EXISTS study_questions (
	id TEXT PRIMARY KEY,
	copy_id TEXT NOT NULL,
	question_no INTEGER NOT NULL DEFAULT 0,
	label TEXT NOT NULL DEFAULT '',
	prompt_text TEXT NOT NULL DEFAULT '',
	prompt_hi TEXT NOT NULL DEFAULT '',
	marks INTEGER NOT NULL DEFAULT 0,
	word_limit INTEGER NOT NULL DEFAULT 0,
	answer_text TEXT NOT NULL DEFAULT '',
	source_pages_json TEXT NOT NULL DEFAULT '[]',
	status TEXT NOT NULL DEFAULT '',
	feedback_json TEXT NOT NULL DEFAULT '',
	analysis_json TEXT NOT NULL DEFAULT '',
	metadata_json TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_study_questions_copy ON study_questions(copy_id, question_no);
CREATE TABLE IF NOT EXISTS study_analyses (
	id TEXT PRIMARY KEY,
	copy_id TEXT NOT NULL,
	scope_type TEXT NOT NULL DEFAULT '',
	scope_id TEXT NOT NULL DEFAULT '',
	dimension_key TEXT NOT NULL DEFAULT '',
	provider_id TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	result_json TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_study_analyses_copy ON study_analyses(copy_id, scope_type, dimension_key);
CREATE TABLE IF NOT EXISTS study_batches (
	id TEXT PRIMARY KEY,
	job_id TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	stage TEXT NOT NULL DEFAULT '',
	provider_id TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	parallelism INTEGER NOT NULL DEFAULT 0,
	force_rerun INTEGER NOT NULL DEFAULT 0,
	total INTEGER NOT NULL DEFAULT 0,
	completed INTEGER NOT NULL DEFAULT 0,
	failed INTEGER NOT NULL DEFAULT 0,
	started_at TIMESTAMP,
	finished_at TIMESTAMP,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL
);
CREATE TABLE IF NOT EXISTS study_batch_items (
	batch_id TEXT NOT NULL,
	copy_id TEXT NOT NULL,
	stage TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	error_kind TEXT NOT NULL DEFAULT '',
	attempt INTEGER NOT NULL DEFAULT 0,
	cache_hit INTEGER NOT NULL DEFAULT 0,
	api_calls INTEGER NOT NULL DEFAULT 0,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	started_at TIMESTAMP,
	finished_at TIMESTAMP,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	PRIMARY KEY(batch_id, copy_id, stage)
);
CREATE INDEX IF NOT EXISTS idx_study_batch_items_batch ON study_batch_items(batch_id, status);
`)
	if err != nil {
		return err
	}
	for _, stmt := range schemaColumnMigrations {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

var schemaColumnMigrations = []string{
	`ALTER TABLE jobs ADD COLUMN stage TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE jobs ADD COLUMN progress REAL NOT NULL DEFAULT 0`,
	`ALTER TABLE jobs ADD COLUMN current_step INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE jobs ADD COLUMN total_steps INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE jobs ADD COLUMN eta_seconds INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE jobs ADD COLUMN progress_mode TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE jobs ADD COLUMN completed_units INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE jobs ADD COLUMN total_units INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE jobs ADD COLUMN unit_label TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE jobs ADD COLUMN progress_started_at TIMESTAMP`,
	`ALTER TABLE jobs ADD COLUMN progress_ends_at TIMESTAMP`,
	`ALTER TABLE jobs ADD COLUMN finished_at TIMESTAMP`,
	`ALTER TABLE study_batches ADD COLUMN job_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE study_batches ADD COLUMN provider_id TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE study_batches ADD COLUMN model TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE study_batches ADD COLUMN parallelism INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batches ADD COLUMN force_rerun INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batches ADD COLUMN started_at TIMESTAMP`,
	`ALTER TABLE study_batches ADD COLUMN finished_at TIMESTAMP`,
	`ALTER TABLE study_batches ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batch_items ADD COLUMN error_kind TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE study_batch_items ADD COLUMN attempt INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batch_items ADD COLUMN cache_hit INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batch_items ADD COLUMN api_calls INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batch_items ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batch_items ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batch_items ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_batch_items ADD COLUMN started_at TIMESTAMP`,
	`ALTER TABLE study_batch_items ADD COLUMN finished_at TIMESTAMP`,
	`ALTER TABLE study_batch_items ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE study_copies ADD COLUMN metadata_json TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE study_questions ADD COLUMN metadata_json TEXT NOT NULL DEFAULT ''`,
}
