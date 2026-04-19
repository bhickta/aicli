"""Repository class for the UPSC Analyze domain."""
from pathlib import Path
from typing import List, Dict, Any, Optional
import sqlite3
import json

from aicli.domains.analyze.database import AnalyzeDB
from aicli.server.schemas.analyze_schemas import (
    PDFListItemDTO, 
    ProcessingStatusDTO, 
    PageDTO, 
    AnswerDTO, 
    AnswerDimensionDTO, 
    AggregationDTO
)

class AnalyzeRepository:
    """Repository handling all database operations for the Analyze domain."""

    def __init__(self, db_path: Path):
        self._db = AnalyzeDB(db_path)

    def get_pdf_list(self, data_dir: Path) -> List[PDFListItemDTO]:
        """Fetch list of PDFs and their processing status."""
        db_pdfs = []
        with self._db._get_conn() as conn:
            # Query processed PDFs
            rows = conn.execute(
                "SELECT pdf_file as filename, count(*) as page_count FROM pages GROUP BY pdf_file ORDER BY pdf_file"
            ).fetchall()
            
            processed_names = set()
            for i, p in enumerate(rows):
                filename = p["filename"]
                processed_names.add(filename)
                progress = self._db.get_pdf_progress(filename)
                db_pdfs.append(PDFListItemDTO(
                    id=i + 1,
                    filename=filename,
                    page_count=p["page_count"],
                    progress=progress
                ))
            
            # Include uploaded but unprocessed PDFs
            if data_dir.exists():
                idx = len(db_pdfs) + 1
                for child in data_dir.glob("*.pdf"):
                    if child.name not in processed_names:
                        db_pdfs.append(PDFListItemDTO(
                            id=idx,
                            filename=child.name,
                            page_count=0,
                            progress={"1": "pending", "2": "pending", "3": "pending", "4": "pending", "5": "pending"}
                        ))
                        idx += 1
        
        db_pdfs.sort(key=lambda x: x.filename)
        return db_pdfs

    def get_status_metrics(self) -> ProcessingStatusDTO:
        """Fetch high-level processing metrics."""
        status = self._db.get_processing_status()
        return ProcessingStatusDTO(
            total_pdfs=status.get("total_pdfs", 0),
            total_pages=status.get("total_pages", 0),
            classified_pages=status.get("classified_pages", 0),
            errors={"OCR": status.get("errors", {}).get("OCR", 0)}
        )

    def get_pdf_pages(self, pdf_id: int) -> List[PageDTO]:
        """Fetch all pages for a specific PDF ID (based on sorted filename index)."""
        pdfs = self._db.get_all_pdfs()
        if pdf_id < 1 or pdf_id > len(pdfs):
            raise ValueError(f"PDF ID {pdf_id} not found")
        
        pdf_name = pdfs[pdf_id - 1]
        pages = self._db.get_pages_for_pdf(pdf_name)
        return [PageDTO(**p) for p in pages]

    def get_pdf_answers(self, pdf_id: int) -> List[AnswerDTO]:
        """Fetch segmented answers for a PDF ID."""
        pdfs = self._db.get_all_pdfs()
        if pdf_id < 1 or pdf_id > len(pdfs):
            raise ValueError(f"PDF ID {pdf_id} not found")
        
        pdf_name = pdfs[pdf_id - 1]
        with self._db._get_conn() as conn:
            rows = conn.execute(
                "SELECT id, question_number, question_directive, question_text, raw_text, page_ids "
                "FROM answers WHERE pdf_file = ? ORDER BY CAST(question_number AS INTEGER)",
                (pdf_name,)
            ).fetchall()
            return [AnswerDTO(**dict(r)) for r in rows]

    def get_answer_dimensions(self, answer_id: int) -> List[AnswerDimensionDTO]:
        """Fetch dimension analysis results for an answer."""
        with self._db._get_conn() as conn:
            rows = conn.execute(
                "SELECT dimension_name, result_json FROM answer_dimensions WHERE answer_id = ?",
                (answer_id,)
            ).fetchall()
            
            results = []
            for r in rows:
                row = dict(r)
                if row.get("result_json"):
                    try:
                        row["result_json"] = json.loads(row["result_json"])
                    except:
                        pass
                results.append(AnswerDimensionDTO(**row))
            return results

    def get_all_aggregations(self) -> List[AggregationDTO]:
        """Fetch all cross-PDF aggregations."""
        rows = self._db.get_all_aggregations()
        results = []
        for r in rows:
            try:
                results.append(AggregationDTO(
                    dimension_name=r["dimension_name"],
                    answer_count=r["answer_count"],
                    aggregation_json=json.loads(r["aggregation_json"])
                ))
            except:
                pass
        return results

    def reset_pipeline(self, step: int):
        """Reset the pipeline from a specific step."""
        self._db.reset_from_step(step)

    def retry_errors(self) -> int:
        """Clear transcription errors to allow retrying."""
        conn = self._db._get_conn()
        cur = conn.execute(
            "UPDATE pages SET transcription = NULL, processed = 0 "
            "WHERE transcription LIKE '[TRANSCRIPTION_ERROR%'"
        )
        conn.commit()
        return cur.rowcount

    def delete_pdf(self, pdf_file: str):
        """Delete all data associated with a PDF."""
        self._db.delete_pdf_data(pdf_file)

    def close(self):
        """Close the database connection."""
        self._db.close()
