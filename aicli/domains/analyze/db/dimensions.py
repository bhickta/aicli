from datetime import datetime, timezone

class DimensionMixin:
    """Operations related to dimension analysis and aggregations."""
    
    def get_unanalyzed_answers(self, dimension_name: str) -> list[dict]:
        conn = self._get_conn()
        rows = conn.execute("""
            SELECT a.* FROM answers a 
            WHERE a.id NOT IN (
              SELECT answer_id FROM answer_dimensions 
              WHERE dimension_name = ? AND result_json NOT LIKE '%"error":%'
            ) ORDER BY a.pdf_file, a.question_number
        """, (dimension_name,)).fetchall()
        return [dict(r) for r in rows]

    def insert_dimension_result(self, answer_id: int, dimension_name: str, result_json: str):
        conn = self._get_conn()
        now = datetime.now(timezone.utc).isoformat()
        conn.execute(
            "INSERT OR REPLACE INTO answer_dimensions "
            "(answer_id, dimension_name, result_json, processed_at) VALUES (?, ?, ?, ?)",
            (answer_id, dimension_name, result_json, now),
        )
        conn.commit()

    def get_dimension_results(self, dimension_name: str) -> list[dict]:
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
        conn = self._get_conn()
        row = conn.execute(
            "SELECT COUNT(*) as cnt FROM answer_dimensions WHERE dimension_name = ?",
            (dimension_name,),
        ).fetchone()
        return row["cnt"]

    def insert_aggregation(self, dimension_name: str, aggregation_json: str, answer_count: int):
        conn = self._get_conn()
        now = datetime.now(timezone.utc).isoformat()
        conn.execute(
            "INSERT OR REPLACE INTO dimension_aggregations "
            "(dimension_name, aggregation_json, generated_at, answer_count) VALUES (?, ?, ?, ?)",
            (dimension_name, aggregation_json, now, answer_count),
        )
        conn.commit()

    def get_aggregation(self, dimension_name: str) -> dict | None:
        conn = self._get_conn()
        row = conn.execute(
            "SELECT * FROM dimension_aggregations WHERE dimension_name = ?",
            (dimension_name,),
        ).fetchone()
        return dict(row) if row else None

    def get_all_aggregations(self) -> list[dict]:
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM dimension_aggregations ORDER BY dimension_name"
        ).fetchall()
        return [dict(r) for r in rows]
