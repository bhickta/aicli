package storage

import (
	"database/sql"
	"net/url"

	_ "modernc.org/sqlite"
)

const sqliteBusyTimeoutMS = "10000"

// OpenSQLite opens the app database with settings that avoid SQLite writer
// contention from concurrent job progress updates.
func OpenSQLite(path string) (*sql.DB, error) {
	values := url.Values{}
	values.Add("_pragma", "busy_timeout="+sqliteBusyTimeoutMS)
	values.Add("_pragma", "journal_mode(WAL)")
	values.Add("_pragma", "synchronous(NORMAL)")
	values.Add("_pragma", "foreign_keys(ON)")

	dsnURL := url.URL{Scheme: "file", Path: path, RawQuery: values.Encode()}
	dsn := dsnURL.String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}
