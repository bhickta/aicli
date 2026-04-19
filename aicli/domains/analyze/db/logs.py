from datetime import datetime, timezone

class LogMixin:
    """Operations related to processing logs and system reset."""
    
    def log_processing(self, pdf_file: str | None, step: str, status: str, error: str | None = None):
        conn = self._get_conn()
        now = datetime.now(timezone.utc).isoformat()
        conn.execute(
            "INSERT INTO processing_log (pdf_file, step, status, error, timestamp) "
            "VALUES (?, ?, ?, ?, ?)",
            (pdf_file, step, status, error, now),
        )
        conn.commit()

    def get_processing_status(self) -> dict:
        conn = self._get_conn()
        status = {}

        status["total_pdfs"] = self.get_pdf_count()

        row = conn.execute("SELECT COUNT(*) as cnt FROM pages").fetchone()
        status["total_pages"] = row["cnt"]

        row = conn.execute("SELECT COUNT(*) as cnt FROM pages WHERE classification IS NOT NULL").fetchone()
        status["classified_pages"] = row["cnt"]

        row = conn.execute("SELECT COUNT(*) as cnt FROM pages WHERE transcription IS NOT NULL").fetchone()
        status["transcribed_pages"] = row["cnt"]

        row = conn.execute("SELECT COUNT(*) as cnt FROM answers").fetchone()
        status["total_answers"] = row["cnt"]

        dim_rows = conn.execute("SELECT dimension_name, COUNT(*) as cnt FROM answer_dimensions GROUP BY dimension_name").fetchall()
        status["dimensions"] = {r["dimension_name"]: r["cnt"] for r in dim_rows}

        agg_rows = conn.execute("SELECT dimension_name, answer_count FROM dimension_aggregations").fetchall()
        status["aggregations"] = {r["dimension_name"]: r["answer_count"] for r in agg_rows}

        err_rows = conn.execute("SELECT step, COUNT(*) as cnt FROM processing_log WHERE status = 'error' GROUP BY step").fetchall()
        status["errors"] = {r["step"]: r["cnt"] for r in err_rows}

        return status

    def reset_from_step(self, step_number: int):
        conn = self._get_conn()
        if step_number <= 6: conn.execute("DELETE FROM dimension_aggregations")
        if step_number <= 5: conn.execute("DELETE FROM answer_dimensions")
        if step_number <= 4: conn.execute("DELETE FROM answers")
        if step_number <= 3: conn.execute("UPDATE pages SET classification = NULL")
        if step_number <= 2: conn.execute("UPDATE pages SET transcription = NULL, processed = 0")
        if step_number <= 1: conn.execute("DELETE FROM pages")

        conn.execute(
            "INSERT INTO processing_log (pdf_file, step, status, timestamp) VALUES (?, ?, ?, ?)",
            (None, f"reset_from_{step_number}", "done", datetime.now(timezone.utc).isoformat()),
        )
        conn.commit()

    def delete_pdf_data(self, pdf_file: str):
        conn = self._get_conn()
        conn.execute("""
            DELETE FROM answer_dimensions 
            WHERE answer_id IN (SELECT id FROM answers WHERE pdf_file = ?)
        """, (pdf_file,))
        conn.execute("DELETE FROM answers WHERE pdf_file = ?", (pdf_file,))
        conn.execute("DELETE FROM pages WHERE pdf_file = ?", (pdf_file,))
        conn.execute("DELETE FROM processing_log WHERE pdf_file = ?", (pdf_file,))
        conn.commit()
