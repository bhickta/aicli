"""Step 4: Segment transcribed pages into answer units by question number.

Groups pages by question boundaries. Each question + its continuation
pages = one answer unit stored in the answers table.
"""
import json
from pathlib import Path

from aicli.domains.analyze.database import AnalyzeDB
from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.analyze.config_loader import AnalyzeConfig


class AnswerSegmenterService:
    """Group transcribed pages into answer units by question number."""

    def __init__(self, provider: LMStudioProvider, config: AnalyzeConfig):
        self.provider = provider
        self.config = config

    def segment_pdf(self, pdf_file: str, db: AnalyzeDB, allow_reasoning: bool = True) -> int:
        """Segment all pages of a PDF into answer units.

        Args:
            pdf_file: The PDF filename (as stored in DB).
            db: Database instance.

        Returns:
            Count of answer units created.
        """
        # Get all transcribed answer/continuation pages for this PDF
        pages = db.get_pages_for_pdf(pdf_file)
        content_pages = [
            p for p in pages
            if p["classification"] in ("answer", "continuation")
            and p["transcription"]
            and not p["transcription"].startswith("[TRANSCRIPTION_ERROR")
        ]

        if not content_pages:
            return 0

        # Build the pages text for the segmentation prompt
        pages_text_parts = []
        for p in content_pages:
            pages_text_parts.append(
                f"--- PAGE {p['page_number']} (classified: {p['classification']}) ---\n"
                f"{p['transcription']}\n"
            )
        pages_text = "\n".join(pages_text_parts)

        # Get segmentation prompt and inject pages
        prompt_template = self.config.segmentation_prompt
        prompt = prompt_template.replace("{pages_text}", pages_text)

        # Call LM Studio for segmentation
        try:
            segments = self.provider.complete_text_json(
                prompt=prompt,
                temperature=self.config.temperature,
                max_tokens=8192,  # Let the model think as much as it wants
                max_retries=self.config.max_retries,
                retry_backoff_base=self.config.retry_backoff_base,
                allow_reasoning=allow_reasoning,
            )
        except Exception as e:
            db.log_processing(pdf_file, "segmentation", "error", str(e))
            # Fallback: treat each "answer" page as a separate answer
            segments = self._fallback_segmentation(content_pages)

        # Handle case where the model returns a dict with a list inside
        if isinstance(segments, dict):
            # Try common keys
            for key in ("answers", "segments", "questions", "data"):
                if key in segments and isinstance(segments[key], list):
                    segments = segments[key]
                    break
            else:
                segments = [segments]

        if not isinstance(segments, list):
            segments = [segments]

        # Extract candidate name from cover page if available
        candidate_name = self._extract_candidate_name(pages, db)

        # Create answer records
        count = 0
        for seg in segments:
            if not isinstance(seg, dict):
                continue

            start_page = seg.get("start_page")
            end_page = seg.get("end_page", start_page)

            if start_page is None:
                continue

            # Collect page IDs and concatenate transcriptions
            answer_page_ids = []
            raw_text_parts = []
            for p in content_pages:
                if start_page <= p["page_number"] <= (end_page or start_page):
                    answer_page_ids.append(p["id"])
                    raw_text_parts.append(p["transcription"])

            if not raw_text_parts:
                continue

            raw_text = "\n\n".join(raw_text_parts)

            db.insert_answer(
                pdf_file=pdf_file,
                candidate_name=candidate_name,
                question_number=seg.get("question_number"),
                question_text=seg.get("question_text"),
                question_directive=seg.get("question_directive"),
                word_limit=seg.get("word_limit"),
                raw_text=raw_text,
                page_ids=answer_page_ids,
            )
            count += 1

        db.log_processing(pdf_file, "segmentation", "done")
        return count

    def segment_all(self, db: AnalyzeDB, progress=None, task_id=None, 
                    allow_reasoning: bool = True, pdf_paths: list[Path] | None = None) -> int:
        """Segment unsegmented PDFs or specific paths.

        Returns:
            Total answer units created.
        """
        if pdf_paths:
            # pdf_paths are local paths, we need filenames for the DB
            pdfs = [p.name for p in pdf_paths]
        else:
            pdfs = db.get_unsegmented_pdfs()
            
        total = 0

        for pdf_file in pdfs:
            count = self.segment_pdf(pdf_file, db, allow_reasoning=allow_reasoning)
            total += count
            if progress and task_id is not None:
                progress.advance(task_id)

        return total

    def _extract_candidate_name(self, pages: list[dict], db: AnalyzeDB, allow_reasoning: bool = True) -> str | None:
        """Try to extract candidate name from the cover page."""
        cover_pages = [p for p in pages if p.get("classification") == "cover"]
        if not cover_pages:
            return None

        # If cover page has a transcription, try to parse the name
        cover = cover_pages[0]
        if not cover.get("transcription"):
            # Transcribe the cover page for name extraction
            try:
                result = self.provider.describe_image(
                    image_path=cover["image_path"],
                    prompt="Extract ONLY the candidate's name from this UPSC cover page. Return just the name, nothing else.",
                    max_size=self.config.image_max_size,
                    temperature=0.1,
                    max_tokens=100,
                    max_retries=2,
                    allow_reasoning=allow_reasoning,
                )
                return result.strip() if result else None
            except Exception:
                return None

        return None

    def _fallback_segmentation(self, content_pages: list[dict]) -> list[dict]:
        """Fallback: treat each 'answer' page as starting a new answer,
        with subsequent 'continuation' pages appended to it."""
        segments = []
        current = None

        for p in content_pages:
            if p["classification"] == "answer":
                if current:
                    segments.append(current)
                current = {
                    "question_number": f"Q.{len(segments) + 1}",
                    "question_text": None,
                    "question_directive": None,
                    "word_limit": None,
                    "start_page": p["page_number"],
                    "end_page": p["page_number"],
                }
            elif p["classification"] == "continuation" and current:
                current["end_page"] = p["page_number"]

        if current:
            segments.append(current)

        return segments
