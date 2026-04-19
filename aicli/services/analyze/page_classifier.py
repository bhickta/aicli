"""Step 3: Classify each page from its OCR transcription text.

Classifications: cover, evaluation, answer, continuation, blank.
Uses text-only LLM calls (fast) since OCR is already done in Step 2.
Parallel-safe — each call is independent.
"""

from concurrent.futures import ThreadPoolExecutor, as_completed

from aicli.domains.analyze.database import AnalyzeDB
from aicli.core.interfaces import ImageVisionProvider
from aicli.services.analyze.config_loader import AnalyzeConfig
from aicli.config import config as app_config

from pydantic import BaseModel, Field
from enum import Enum
from langchain_core.prompts import PromptTemplate

class PageClass(str, Enum):
    cover = "cover"
    evaluation = "evaluation"
    answer = "answer"
    continuation = "continuation"
    blank = "blank"

class PageClassificationSchema(BaseModel):
    classification: PageClass = Field(description="The classification of the page: cover, evaluation, answer, continuation, or blank.")

class PageClassifierService:
    """Classify pages using their OCR'd text (text-only LLM calls)."""

    def __init__(self, provider: ImageVisionProvider, config: AnalyzeConfig):
        self.provider = provider
        self.config = config

    def classify_page(self, page_row: dict, allow_reasoning: bool = True) -> str:
        """Classify a single page from its transcription text. Returns classification string."""
        prompt = self.config.classification_prompt
        max_tokens = app_config.analyze_max_tokens  # Generous ceiling to prevent cutoffs for models that ignore the reasoning=False flag

        if not allow_reasoning:
            prompt = f"[SHORT RESPONSE MODE]\nRespond ONLY with the single word classification tag.\nDO NOT think step-by-step. DO NOT explain.\n\n{prompt}"

        transcription = page_row.get("transcription") or ""

        # If transcription is empty or error, it's likely blank
        if not transcription.strip() or transcription.startswith(
            "[TRANSCRIPTION_ERROR"
        ):
            return "blank"

        prompt_template = PromptTemplate.from_template("{prompt}\n\n---\nPAGE TEXT:\n{text}")
        full_prompt = prompt_template.format(prompt=prompt, text=transcription)

        try:
            result = self.provider.structured_invoke(
                schema=PageClassificationSchema,
                prompt=full_prompt,
                allow_reasoning=allow_reasoning,
            )
            return result.classification.value
        except Exception as e:
            # Safe default
            return "answer"

    def classify_batch(
        self,
        db: AnalyzeDB,
        workers: int = 4,
        progress=None,
        task_id=None,
        allow_reasoning: bool = True,
    ) -> tuple[int, int]:
        """Classify all unclassified pages in parallel (text-only).

        Returns:
            (success_count, error_count)
        """
        pages = db.get_unclassified_pages()
        if not pages:
            return 0, 0

        count = 0
        errors = 0
        first_error_shown = False

        def _process_one(page_row):
            classification = self.classify_page(
                page_row, allow_reasoning=allow_reasoning
            )
            db.update_classification(page_row["id"], classification)
            # Log success
            if progress:
                progress.console.print(
                    f"[SUCCESS] [PAGE:{page_row['page_number']}] Classified as {classification}: {page_row['pdf_file']}"
                )
            return page_row["id"], classification

        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = {executor.submit(_process_one, p): p for p in pages}
            for future in as_completed(futures):
                page = futures[future]
                try:
                    page_id, cls = future.result()
                    count += 1
                    if progress and task_id is not None:
                        progress.advance(task_id)
                except Exception as e:
                    errors += 1
                    db.log_processing(
                        page["pdf_file"], "classification", "error", str(e)
                    )
                    # Standardized error log
                    if progress:
                        progress.console.print(
                            f"[ERROR] [PAGE:{page['page_number']}] Classification failed: {str(e)[:100]}"
                        )
                    if progress and task_id is not None:
                        progress.advance(task_id)

        return count, errors
