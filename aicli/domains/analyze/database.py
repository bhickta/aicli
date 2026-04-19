"""SQLite database manager for the UPSC analyze pipeline.

All tables are created idempotently. Every pipeline step reads/writes
through this single class, keeping SQL out of service code.
"""
import json
import sqlite3
import threading
from datetime import datetime, timezone
from pathlib import Path


class AnalyzeDB:
    """Thread-safe SQLite wrapper for the full analyze pipeline."""

    def __init__(self, db_path: Path):
        self._db_path = db_path
        self._local = threading.local()
        # Create tables on the main thread immediately
        self._create_tables(self._get_conn())

    # ------------------------------------------------------------------
    # Connection management (one connection per thread)
    # ------------------------------------------------------------------
    def _get_conn(self) -> sqlite3.Connection:
        """Return a per-thread connection (created lazily)."""
        conn = getattr(self._local, "conn", None)
        if conn is None:
            conn = sqlite3.connect(str(self._db_path), timeout=30)
            conn.row_factory = sqlite3.Row
            conn.execute("PRAGMA journal_mode=WAL")
            conn.execute("PRAGMA busy_timeout=5000")
            self._local.conn = conn
        return conn

    # ------------------------------------------------------------------
    # Schema
    # ------------------------------------------------------------------
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

    # ------------------------------------------------------------------
    # Pages
    # ------------------------------------------------------------------
    def insert_page(self, pdf_file: str, page_number: int, image_path: str) -> int:
        """Insert a page record. Returns row id. Skips if duplicate."""
        conn = self._get_conn()
        try:
            cur = conn.execute(
                "INSERT INTO pages (pdf_file, page_number, image_path) VALUES (?, ?, ?)",
                (pdf_file, page_number, image_path),
            )
            conn.commit()
            return cur.lastrowid
        except sqlite3.IntegrityError:
            # Already exists — return existing id
            row = conn.execute(
                "SELECT id FROM pages WHERE pdf_file = ? AND page_number = ?",
                (pdf_file, page_number),
            ).fetchone()
            return row["id"] if row else -1

    def get_pages_for_pdf(self, pdf_file: str) -> list[dict]:
        """Get all pages for a PDF, ordered by page number."""
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM pages WHERE pdf_file = ? ORDER BY page_number",
            (pdf_file,),
        ).fetchall()
        return [dict(r) for r in rows]

    def get_unclassified_pages(self) -> list[dict]:
        """Get pages that haven't been classified yet."""
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM pages WHERE classification IS NULL ORDER BY pdf_file, page_number"
        ).fetchall()
        return [dict(r) for r in rows]

    def update_classification(self, page_id: int, classification: str):
        """Set the classification for a page."""
        conn = self._get_conn()
        conn.execute(
            "UPDATE pages SET classification = ? WHERE id = ?",
            (classification, page_id),
        )
        conn.commit()

    def get_untranscribed_pages(self) -> list[dict]:
        """Get ALL pages that haven't been transcribed yet (for OCR-first pipeline)."""
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM pages WHERE transcription IS NULL ORDER BY pdf_file, page_number"
        ).fetchall()
        return [dict(r) for r in rows]

    def update_transcription(self, page_id: int, transcription: str):
        """Set the transcription for a page."""
        conn = self._get_conn()
        conn.execute(
            "UPDATE pages SET transcription = ?, processed = 1 WHERE id = ?",
            (transcription, page_id),
        )
        conn.commit()

    def get_pdf_count(self) -> int:
        """Count distinct PDF files in the database."""
        conn = self._get_conn()
        row = conn.execute("SELECT COUNT(DISTINCT pdf_file) as cnt FROM pages").fetchone()
        return row["cnt"]

    def get_all_pdfs(self) -> list[str]:
        """Get all distinct PDF file paths."""
        conn = self._get_conn()
        rows = conn.execute("SELECT DISTINCT pdf_file FROM pages ORDER BY pdf_file").fetchall()
        return [r["pdf_file"] for r in rows]

    # ------------------------------------------------------------------
    # Answers
    # ------------------------------------------------------------------
    def get_unsegmented_pdfs(self) -> list[str]:
        """Get PDFs that have transcribed pages but no segmented answers yet."""
        conn = self._get_conn()
        # PDFs with at least one transcribed answer page that don't have answers yet
        rows = conn.execute("""
            SELECT DISTINCT p.pdf_file FROM pages p
            WHERE p.classification IN ('answer', 'continuation')
              AND p.transcription IS NOT NULL
              AND p.pdf_file NOT IN (SELECT DISTINCT pdf_file FROM answers)
            ORDER BY p.pdf_file
        """).fetchall()
        return [r["pdf_file"] for r in rows]

    def insert_answer(
        self,
        pdf_file: str,
        candidate_name: str | None,
        question_number: str | None,
        question_text: str | None,
        question_directive: str | None,
        word_limit: int | None,
        raw_text: str,
        page_ids: list[int],
    ) -> int:
        """Insert an answer record. Returns row id."""
        conn = self._get_conn()
        cur = conn.execute(
            "INSERT INTO answers (pdf_file, candidate_name, question_number, "
            "question_text, question_directive, word_limit, raw_text, page_ids, segmentation_done) "
            "VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)",
            (
                pdf_file,
                candidate_name,
                question_number,
                question_text,
                question_directive,
                word_limit,
                raw_text,
                json.dumps(page_ids),
            ),
        )
        conn.commit()
        return cur.lastrowid

    def get_all_answers(self) -> list[dict]:
        """Get all answers."""
        conn = self._get_conn()
        rows = conn.execute("SELECT * FROM answers ORDER BY pdf_file, question_number").fetchall()
        return [dict(r) for r in rows]

    def get_answer_by_id(self, answer_id: int) -> dict | None:
        """Get a single answer by id."""
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM answers WHERE id = ?", (answer_id,)).fetchone()
        return dict(row) if row else None

    # ------------------------------------------------------------------
    # Dimensions
    # ------------------------------------------------------------------
    def get_unanalyzed_answers(self, dimension_name: str) -> list[dict]:
        """Get answers that haven't been analyzed for a specific dimension."""
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT a.* FROM answers a "
            "WHERE a.id NOT IN ("
            "  SELECT answer_id FROM answer_dimensions WHERE dimension_name = ?"
            ") ORDER BY a.pdf_file, a.question_number",
            (dimension_name,),
        ).fetchall()
        return [dict(r) for r in rows]

    def insert_dimension_result(self, answer_id: int, dimension_name: str, result_json: str):
        """Insert or replace a dimension analysis result."""
        conn = self._get_conn()
        now = datetime.now(timezone.utc).isoformat()
        conn.execute(
            "INSERT OR REPLACE INTO answer_dimensions "
            "(answer_id, dimension_name, result_json, processed_at) VALUES (?, ?, ?, ?)",
            (answer_id, dimension_name, result_json, now),
        )
        conn.commit()

    def get_dimension_results(self, dimension_name: str) -> list[dict]:
        """Get all results for a dimension, joined with answer metadata."""
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT ad.*, a.pdf_file, a.candidate_name, a.question_number, "
            "a.question_text, a.question_directive "
            "FROM answer_dimensions ad "
            "JOIN answers a ON ad.answer_id = a.id "
            "WHERE ad.dimension_name = ? "
            "ORDER BY a.pdf_file, a.question_number",
            (dimension_name,),
        ).fetchall()
        return [dict(r) for r in rows]

    def get_dimension_count(self, dimension_name: str) -> int:
        """Count how many answers have been analyzed for a dimension."""
        conn = self._get_conn()
        row = conn.execute(
            "SELECT COUNT(*) as cnt FROM answer_dimensions WHERE dimension_name = ?",
            (dimension_name,),
        ).fetchone()
        return row["cnt"]

    # ------------------------------------------------------------------
    # Aggregations
    # ------------------------------------------------------------------
    def insert_aggregation(self, dimension_name: str, aggregation_json: str, answer_count: int):
        """Insert or replace an aggregation result."""
        conn = self._get_conn()
        now = datetime.now(timezone.utc).isoformat()
        conn.execute(
            "INSERT OR REPLACE INTO dimension_aggregations "
            "(dimension_name, aggregation_json, generated_at, answer_count) VALUES (?, ?, ?, ?)",
            (dimension_name, aggregation_json, now, answer_count),
        )
        conn.commit()

    def get_aggregation(self, dimension_name: str) -> dict | None:
        """Get aggregation for a dimension."""
        conn = self._get_conn()
        row = conn.execute(
            "SELECT * FROM dimension_aggregations WHERE dimension_name = ?",
            (dimension_name,),
        ).fetchone()
        return dict(row) if row else None

    def get_all_aggregations(self) -> list[dict]:
        """Get all aggregations."""
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM dimension_aggregations ORDER BY dimension_name"
        ).fetchall()
        return [dict(r) for r in rows]

    # ------------------------------------------------------------------
    # Processing Log
    # ------------------------------------------------------------------
    def log_processing(self, pdf_file: str | None, step: str, status: str, error: str | None = None):
        """Write a processing log entry."""
        conn = self._get_conn()
        now = datetime.now(timezone.utc).isoformat()
        conn.execute(
            "INSERT INTO processing_log (pdf_file, step, status, error, timestamp) "
            "VALUES (?, ?, ?, ?, ?)",
            (pdf_file, step, status, error, now),
        )
        conn.commit()

    def get_processing_status(self) -> dict:
        """Get a summary of processing status across all steps."""
        conn = self._get_conn()
        status = {}

        # PDF count
        status["total_pdfs"] = self.get_pdf_count()

        # Page counts
        row = conn.execute("SELECT COUNT(*) as cnt FROM pages").fetchone()
        status["total_pages"] = row["cnt"]

        row = conn.execute(
            "SELECT COUNT(*) as cnt FROM pages WHERE classification IS NOT NULL"
        ).fetchone()
        status["classified_pages"] = row["cnt"]

        row = conn.execute(
            "SELECT COUNT(*) as cnt FROM pages WHERE transcription IS NOT NULL"
        ).fetchone()
        status["transcribed_pages"] = row["cnt"]

        # Answer count
        row = conn.execute("SELECT COUNT(*) as cnt FROM answers").fetchone()
        status["total_answers"] = row["cnt"]

        # Dimension counts
        dim_rows = conn.execute(
            "SELECT dimension_name, COUNT(*) as cnt FROM answer_dimensions GROUP BY dimension_name"
        ).fetchall()
        status["dimensions"] = {r["dimension_name"]: r["cnt"] for r in dim_rows}

        # Aggregation counts
        agg_rows = conn.execute(
            "SELECT dimension_name, answer_count FROM dimension_aggregations"
        ).fetchall()
        status["aggregations"] = {r["dimension_name"]: r["answer_count"] for r in agg_rows}

        # Errors
        err_rows = conn.execute(
            "SELECT step, COUNT(*) as cnt FROM processing_log WHERE status = 'error' GROUP BY step"
        ).fetchall()
        status["errors"] = {r["step"]: r["cnt"] for r in err_rows}

        return status

    # ------------------------------------------------------------------
    # Reset
    # ------------------------------------------------------------------
    def reset_from_step(self, step_number: int):
        """Reset all data from a given step onwards.

        Steps: 1=images, 2=transcription, 3=classification,
               4=segmentation, 5=dimensions, 6=aggregation
        """
        conn = self._get_conn()

        if step_number <= 6:
            conn.execute("DELETE FROM dimension_aggregations")
        if step_number <= 5:
            conn.execute("DELETE FROM answer_dimensions")
        if step_number <= 4:
            conn.execute("DELETE FROM answers")
        if step_number <= 3:
            conn.execute("UPDATE pages SET classification = NULL")
        if step_number <= 2:
            conn.execute("UPDATE pages SET transcription = NULL, processed = 0")
        if step_number <= 1:
            conn.execute("DELETE FROM pages")

        conn.execute(
            "INSERT INTO processing_log (pdf_file, step, status, timestamp) VALUES (?, ?, ?, ?)",
            (None, f"reset_from_{step_number}", "done", datetime.now(timezone.utc).isoformat()),
        )
        conn.commit()

    def close(self):
        """Close the connection for the current thread."""
        conn = getattr(self._local, "conn", None)
        if conn:
            conn.close()
            self._local.conn = None
