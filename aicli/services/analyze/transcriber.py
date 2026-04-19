"""Step 3: Transcribe answer pages using LM Studio vision model.

Vision-only OCR — no PaddleOCR. The vision model handles handwritten
cursive English directly from page images. Fully parallelizable.
"""

from concurrent.futures import ThreadPoolExecutor, as_completed

from aicli.domains.analyze.database import AnalyzeDB
from aicli.providers.ollama import OllamaProvider
from aicli.services.analyze.config_loader import AnalyzeConfig


class AnswerTranscriberService:
    """Vision-model-only transcription for handwritten answer pages."""

    def __init__(self, provider: OllamaProvider, config: AnalyzeConfig):
        self.provider = provider
        self.config = config

    def transcribe_page(
        self, page_row: dict, allow_reasoning: bool = True, abort_event=None
    ) -> str:
        """Transcribe a single page using the vision model.

        Returns:
            Raw transcription text.
        """
        prompt = self.config.transcription_prompt
        max_tokens = 4000 if allow_reasoning else 2000

        if not allow_reasoning:
            prompt = f"[SHORT RESPONSE MODE]\nTranscribe ONLY the visible text. DO NOT provide any reasoning, analysis, or explanation.\n\n{prompt}"

        result = self.provider.describe_image(
            image_path=page_row["image_path"],
            prompt=prompt,
            max_size=self.config.image_max_size,
            temperature=self.config.temperature,
            max_tokens=max_tokens,
            max_retries=self.config.max_retries,
            retry_backoff_base=self.config.retry_backoff_base,
            allow_reasoning=allow_reasoning,
            abort_event=abort_event,
        )

        return result

    def transcribe_batch(
        self,
        db: AnalyzeDB,
        workers: int = 4,
        progress=None,
        task_id=None,
        allow_reasoning: bool = True,
        abort_event=None,
    ) -> tuple[int, int]:
        """Transcribe all untranscribed answer/continuation pages in parallel.

        Returns:
            (success_count, error_count)
        """
        pages = db.get_untranscribed_pages()
        if not pages:
            return 0, 0

        count = 0
        errors = 0
        first_error_shown = False

        def _process_one(page_row):
            transcription = self.transcribe_page(
                page_row, allow_reasoning=allow_reasoning, abort_event=abort_event
            )
            db.update_transcription(page_row["id"], transcription)
            # Log success
            if progress:
                progress.console.print(
                    f"[SUCCESS] [PAGE:{page_row['page_number']}] OCR complete: {page_row['pdf_file']}"
                )
            return page_row["id"]

        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = {executor.submit(_process_one, p): p for p in pages}
            for future in as_completed(futures):
                page = futures[future]
                try:
                    future.result()
                    count += 1
                    if progress and task_id is not None:
                        progress.advance(task_id)
                except Exception as e:
                    errors += 1
                    # Log error but continue with remaining pages
                    db.log_processing(
                        page["pdf_file"], "transcription", "error", str(e)
                    )
                    # Standardized error log
                    if progress:
                        progress.console.print(
                            f"[ERROR] [PAGE:{page['page_number']}] Transcription failed: {str(e)[:100]}"
                        )

                    # Mark as needs_review by storing error transcription
                    try:
                        db.update_transcription(
                            page["id"], f"[TRANSCRIPTION_ERROR: {str(e)}]"
                        )
                    except Exception:
                        pass
                    if progress and task_id is not None:
                        progress.advance(task_id)

        return count, errors
