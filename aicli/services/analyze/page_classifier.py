"""Step 2: Classify each page image via LM Studio vision model.

Classifications: cover, evaluation, answer, continuation, blank.
Parallel-safe — each call is independent.
"""
from concurrent.futures import ThreadPoolExecutor, as_completed

from aicli.domains.analyze.database import AnalyzeDB
from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.analyze.config_loader import AnalyzeConfig


VALID_CLASSIFICATIONS = {"cover", "evaluation", "answer", "continuation", "blank"}


class PageClassifierService:
    """Classify page images using vision model."""

    def __init__(self, provider: LMStudioProvider, config: AnalyzeConfig):
        self.provider = provider
        self.config = config

    def classify_page(self, page_row: dict) -> str:
        """Classify a single page image. Returns classification string."""
        prompt = self.config.classification_prompt
        result = self.provider.describe_image(
            image_path=page_row["image_path"],
            prompt=prompt,
            max_size=self.config.image_max_size,
            temperature=self.config.temperature,
            max_tokens=50,  # Classification is a single word
            max_retries=self.config.max_retries,
            retry_backoff_base=self.config.retry_backoff_base,
        )

        # Parse the single-word response
        classification = result.strip().lower().strip('"').strip("'")

        # Validate — fall back to "answer" if unrecognized
        if classification not in VALID_CLASSIFICATIONS:
            # Try to find a valid classification within the response
            for vc in VALID_CLASSIFICATIONS:
                if vc in classification:
                    classification = vc
                    break
            else:
                classification = "answer"  # Safe default

        return classification

    def classify_batch(
        self,
        db: AnalyzeDB,
        workers: int = 4,
        progress=None,
        task_id=None,
    ) -> int:
        """Classify all unclassified pages in parallel.

        Returns:
            Count of pages classified.
        """
        pages = db.get_unclassified_pages()
        if not pages:
            return 0

        count = 0

        def _process_one(page_row):
            classification = self.classify_page(page_row)
            db.update_classification(page_row["id"], classification)
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
                    db.log_processing(
                        page["pdf_file"], "classification", "error", str(e)
                    )
                    if progress and task_id is not None:
                        progress.advance(task_id)

        return count
