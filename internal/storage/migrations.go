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
`)
	if err != nil {
		return err
	}
	for _, stmt := range jobColumnMigrations {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return err
		}
	}
	return nil
}

var jobColumnMigrations = []string{
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
}
