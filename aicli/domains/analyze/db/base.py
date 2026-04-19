import threading
import sqlite3
from pathlib import Path

class BaseSQLite:
    """Thread-safe SQLite wrapper root."""
    def __init__(self, db_path: Path):
        self._db_path = db_path
        self._local = threading.local()
        self._create_tables(self._get_conn())

    def _get_conn(self) -> sqlite3.Connection:
        conn = getattr(self._local, "conn", None)
        if conn is None:
            from aicli.config import config as app_config
            conn = sqlite3.connect(str(self._db_path), timeout=app_config.db_connect_timeout)
            conn.row_factory = sqlite3.Row
            conn.execute("PRAGMA journal_mode=WAL")
            conn.execute(f"PRAGMA busy_timeout={app_config.db_busy_timeout_ms}")
            self._local.conn = conn
        return conn

    def _create_tables(self, conn: sqlite3.Connection):
        conn.executescript("""
            CREATE TABLE IF NOT EXISTS pages (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                pdf_file TEXT NOT NULL,
                page_number INTEGER NOT NULL,
                image_path TEXT NOT NULL,
                classification TEXT,
                transcription TEXT,
                processed INTEGER DEFAULT 0,
                UNIQUE(pdf_file, page_number)
            );

            CREATE TABLE IF NOT EXISTS answers (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                pdf_file TEXT NOT NULL,
                candidate_name TEXT,
                upsc_id TEXT,
                test_code TEXT,
                question_number TEXT,
                question_text TEXT,
                question_directive TEXT,
                word_limit INTEGER,
                raw_text TEXT,
                page_ids TEXT,
                segmentation_done INTEGER DEFAULT 0
            );

            CREATE TABLE IF NOT EXISTS answer_dimensions (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                answer_id INTEGER NOT NULL,
                dimension_name TEXT NOT NULL,
                result_json TEXT NOT NULL,
                processed_at TEXT NOT NULL,
                FOREIGN KEY (answer_id) REFERENCES answers(id),
                UNIQUE(answer_id, dimension_name)
            );

            CREATE TABLE IF NOT EXISTS dimension_aggregations (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                dimension_name TEXT NOT NULL UNIQUE,
                aggregation_json TEXT NOT NULL,
                generated_at TEXT NOT NULL,
                answer_count INTEGER NOT NULL
            );

            CREATE TABLE IF NOT EXISTS processing_log (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                pdf_file TEXT,
                step TEXT NOT NULL,
                status TEXT NOT NULL,
                error TEXT,
                timestamp TEXT NOT NULL
            );
        """)
        conn.commit()

    def close(self):
        conn = getattr(self._local, "conn", None)
        if conn:
            conn.close()
            self._local.conn = None
