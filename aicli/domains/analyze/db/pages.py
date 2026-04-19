import sqlite3

class PageMixin:
    """Operations related to the 'pages' table."""
    def insert_page(self, pdf_file: str, page_number: int, image_path: str) -> int:
        conn = self._get_conn()
        try:
            cur = conn.execute(
                "INSERT INTO pages (pdf_file, page_number, image_path) VALUES (?, ?, ?)",
                (pdf_file, page_number, image_path),
            )
            conn.commit()
            return cur.lastrowid
        except sqlite3.IntegrityError:
            row = conn.execute(
                "SELECT id FROM pages WHERE pdf_file = ? AND page_number = ?",
                (pdf_file, page_number),
            ).fetchone()
            return row["id"] if row else -1

    def get_pages_for_pdf(self, pdf_file: str) -> list[dict]:
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM pages WHERE pdf_file = ? ORDER BY page_number",
            (pdf_file,),
        ).fetchall()
        return [dict(r) for r in rows]

    def get_unclassified_pages(self) -> list[dict]:
        conn = self._get_conn()
        rows = conn.execute(
            "SELECT * FROM pages WHERE classification IS NULL ORDER BY pdf_file, page_number"
        ).fetchall()
        return [dict(r) for r in rows]

    def update_classification(self, page_id: int, classification: str):
        conn = self._get_conn()
        conn.execute(
            "UPDATE pages SET classification = ? WHERE id = ?",
            (classification, page_id),
        )
        conn.commit()

    def get_untranscribed_pages(self) -> list[dict]:
        conn = self._get_conn()
        rows = conn.execute("""
            SELECT * FROM pages 
            WHERE transcription IS NULL 
               OR transcription LIKE '[TRANSCRIPTION_ERROR%'
            ORDER BY pdf_file, page_number
        """).fetchall()
        return [dict(r) for r in rows]

    def update_transcription(self, page_id: int, transcription: str):
        conn = self._get_conn()
        conn.execute(
            "UPDATE pages SET transcription = ?, processed = 1 WHERE id = ?",
            (transcription, page_id),
        )
        conn.commit()

    def get_page(self, page_id: int) -> dict | None:
        conn = self._get_conn()
        row = conn.execute("SELECT * FROM pages WHERE id = ?", (page_id,)).fetchone()
        return dict(row) if row else None

    def get_pdf_count(self) -> int:
        conn = self._get_conn()
        row = conn.execute("SELECT COUNT(DISTINCT pdf_file) as cnt FROM pages").fetchone()
        return row["cnt"]

    def get_all_pdfs(self) -> list[str]:
        conn = self._get_conn()
        rows = conn.execute("SELECT DISTINCT pdf_file FROM pages ORDER BY pdf_file").fetchall()
        return [r["pdf_file"] for r in rows]

    def get_pdf_progress(self, pdf_file: str) -> dict:
        conn = self._get_conn()
        progress = {}

        # Step 1: PDF -> Images
        row = conn.execute("SELECT COUNT(*) as cnt FROM pages WHERE pdf_file = ?", (pdf_file,)).fetchone()
        page_count = row["cnt"]
        progress["1"] = "done" if page_count > 0 else "pending"

        if page_count == 0:
            for s in ["2", "3", "4", "5"]: progress[s] = "pending"
            return progress

        # Step 2: OCR
        row = conn.execute("SELECT COUNT(*) as cnt FROM pages WHERE pdf_file = ? AND transcription IS NOT NULL", (pdf_file,)).fetchone()
        ocr_count = row["cnt"]
        if ocr_count == page_count: progress["2"] = "done"
        elif ocr_count > 0: progress["2"] = "partial"
        else: progress["2"] = "pending"

        # Step 3: Classification
        row = conn.execute("SELECT COUNT(*) as cnt FROM pages WHERE pdf_file = ? AND classification IS NOT NULL", (pdf_file,)).fetchone()
        cls_count = row["cnt"]
        if cls_count == page_count: progress["3"] = "done"
        elif cls_count > 0: progress["3"] = "partial"
        else: progress["3"] = "pending"

        # Step 4: Segmentation
        row = conn.execute("SELECT COUNT(*) as cnt FROM answers WHERE pdf_file = ?", (pdf_file,)).fetchone()
        ans_count = row["cnt"]
        progress["4"] = "done" if ans_count > 0 else "pending"

        # Step 5: Analysis
        if ans_count > 0:
            # Check if all answers have at least one dimension result
            row = conn.execute("""
                SELECT COUNT(DISTINCT answer_id) as cnt FROM answer_dimensions 
                WHERE answer_id IN (SELECT id FROM answers WHERE pdf_file = ?)
            """, (pdf_file,)).fetchone()
            analyzed_count = row["cnt"]
            if analyzed_count == ans_count: progress["5"] = "done"
            elif analyzed_count > 0: progress["5"] = "partial"
            else: progress["5"] = "pending"
        else:
            progress["5"] = "pending"

        return progress
