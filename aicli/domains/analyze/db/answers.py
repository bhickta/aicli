import json

class AnswerMixin:
    """Operations related to the 'answers' table."""
    def get_unsegmented_pdfs(self) -> list[str]:
        conn = self._get_conn()
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
        conn = self._get_conn()
        rows = conn.execute("SELECT * FROM answers ORDER BY pdf_file, question_number").fetchall()
        return [dict(r) for r in rows]

    def get_answer_by_id(self, answer_id: int) -> dict | None:
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM answers WHERE id = ?", (answer_id,)).fetchone()
        return dict(row) if row else None
