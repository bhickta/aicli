"""Repository class for the UPSC Analyze domain."""
import json
from pathlib import Path
from typing import List, Dict, Any, Optional

from aicli.domains.analyze.database import AnalyzeDB
from aicli.server.schemas.analyze_schemas import (
    PDFListItemDTO,
    ProcessingStatusDTO,
    PageDTO,
    AnswerDTO,
    AnswerDimensionDTO,
    AggregationDTO,
)


class AnalyzeRepository:
    """Repository handling all database operations for the Analyze domain."""

    def __init__(self, db_path: Path) -> None:
        self._db = AnalyzeDB(db_path)

    # ── PDF Operations ──────────────────────────────────────────────

    def get_pdf_list(self, data_dir: Path) -> List[PDFListItemDTO]:
        """Fetch list of PDFs and their processing status."""
        processed = self._get_processed_pdfs()
        unprocessed = self._get_unprocessed_pdfs(data_dir, processed)
        combined = processed + unprocessed
        combined.sort(key=lambda x: x.filename)
        return combined

    def delete_pdf(self, pdf_file: str) -> None:
        """Delete all data associated with a PDF."""
        self._db.delete_pdf_data(pdf_file)

    # ── Status ──────────────────────────────────────────────────────

    def get_status_metrics(self) -> ProcessingStatusDTO:
        """Fetch high-level processing metrics."""
        status = self._db.get_processing_status()
        return ProcessingStatusDTO(
            total_pdfs=status.get("total_pdfs", 0),
            total_pages=status.get("total_pages", 0),
            classified_pages=status.get("classified_pages", 0),
            errors=status.get("errors", {}),
        )

    # ── Page Operations ─────────────────────────────────────────────

    def get_pdf_pages(self, pdf_id: int) -> List[PageDTO]:
        """Fetch all pages for a specific PDF ID (based on sorted filename index)."""
        pdf_name = self._resolve_pdf_name(pdf_id)
        pages = self._db.get_pages_for_pdf(pdf_name)
        return [PageDTO(**p) for p in pages]

    # ── Answer Operations ───────────────────────────────────────────

    def get_pdf_answers(self, pdf_id: int) -> List[AnswerDTO]:
        """Fetch segmented answers for a PDF ID."""
        pdf_name = self._resolve_pdf_name(pdf_id)
        with self._db._get_conn() as conn:
            rows = conn.execute(
                "SELECT id, pdf_file, candidate_name, upsc_id, test_code, question_number, "
                "question_directive, question_text, raw_text, page_ids "
                "FROM answers WHERE pdf_file = ? ORDER BY CAST(question_number AS INTEGER)",
                (pdf_name,),
            ).fetchall()
            return [AnswerDTO(**dict(r)) for r in rows]

    def get_answers_for_page(self, page_id: int) -> List[Dict[str, Any]]:
        """Fetch all answers that reference a specific page ID.

        Used by the pipeline service for targeted page re-analysis instead of
        reaching into the DB internals directly.
        """
        with self._db._get_conn() as conn:
            rows = conn.execute("SELECT * FROM answers").fetchall()

        return self._filter_answers_by_page(rows, page_id)

    # ── Dimension Operations ────────────────────────────────────────

    def get_answer_dimensions(self, answer_id: int) -> List[AnswerDimensionDTO]:
        """Fetch dimension analysis results for an answer."""
        with self._db._get_conn() as conn:
            rows = conn.execute(
                "SELECT dimension_name, result_json FROM answer_dimensions WHERE answer_id = ?",
                (answer_id,),
            ).fetchall()

        return [self._parse_dimension_row(r) for r in rows]

    def get_all_aggregations(self) -> List[AggregationDTO]:
        """Fetch all cross-PDF aggregations."""
        rows = self._db.get_all_aggregations()
        results = []
        for r in rows:
            try:
                results.append(AggregationDTO(
                    dimension_name=r["dimension_name"],
                    answer_count=r["answer_count"],
                    aggregation_json=json.loads(r["aggregation_json"]),
                ))
            except (json.JSONDecodeError, KeyError):
                continue
        return results

    # ── Pipeline Control ────────────────────────────────────────────

    def reset_pipeline(self, step: int) -> None:
        """Reset the pipeline from a specific step."""
        self._db.reset_from_step(step)

    def retry_errors(self) -> int:
        """Clear transcription errors to allow retrying."""
        with self._db._get_conn() as conn:
            cur = conn.execute(
                "UPDATE pages SET transcription = NULL, processed = 0 "
                "WHERE transcription LIKE '[TRANSCRIPTION_ERROR%'"
            )
            conn.commit()
            return cur.rowcount

    def close(self) -> None:
        """Close the database connection."""
        self._db.close()

    # ── Private Helpers ─────────────────────────────────────────────

    def _resolve_pdf_name(self, pdf_id: int) -> str:
        """Map a 1-based integer ID to a PDF filename."""
        pdfs = self._db.get_all_pdfs()
        if pdf_id < 1 or pdf_id > len(pdfs):
            raise ValueError(f"PDF ID {pdf_id} not found")
        return pdfs[pdf_id - 1]

    def _get_processed_pdfs(self) -> List[PDFListItemDTO]:
        """Fetch PDFs that already have pages in the database."""
        with self._db._get_conn() as conn:
            rows = conn.execute(
                "SELECT pdf_file as filename, count(*) as page_count "
                "FROM pages GROUP BY pdf_file ORDER BY pdf_file"
            ).fetchall()

        result = []
        for i, row in enumerate(rows):
            filename = row["filename"]
            progress = self._db.get_pdf_progress(filename)
            result.append(PDFListItemDTO(
                id=i + 1,
                filename=filename,
                page_count=row["page_count"],
                progress=progress,
            ))
        return result

    def _get_unprocessed_pdfs(
        self, data_dir: Path, processed: List[PDFListItemDTO]
    ) -> List[PDFListItemDTO]:
        """Find uploaded PDFs that haven't been processed yet."""
        if not data_dir.exists():
            return []

        processed_names = {p.filename for p in processed}
        pending_progress = {"1": "pending", "2": "pending", "3": "pending", "4": "pending", "5": "pending"}
        result = []
        idx = len(processed) + 1

        for child in data_dir.glob("*.pdf"):
            if child.name not in processed_names:
                result.append(PDFListItemDTO(
                    id=idx,
                    filename=child.name,
                    page_count=0,
                    progress=pending_progress,
                ))
                idx += 1
        return result

    def _parse_dimension_row(self, row) -> AnswerDimensionDTO:
        """Parse a raw DB row into a typed DTO, attempting JSON deserialization."""
        data = dict(row)
        if data.get("result_json"):
            try:
                data["result_json"] = json.loads(data["result_json"])
            except json.JSONDecodeError:
                pass  # Keep the raw string
        return AnswerDimensionDTO(**data)

    @staticmethod
    def _filter_answers_by_page(rows, page_id: int) -> List[Dict[str, Any]]:
        """Filter answer rows to those referencing a specific page ID."""
        matched = []
        for r in rows:
            ans = dict(r)
            try:
                ids = json.loads(ans["page_ids"])
                if page_id in ids or str(page_id) in ids:
                    matched.append(ans)
            except (json.JSONDecodeError, KeyError):
                continue
        return matched
