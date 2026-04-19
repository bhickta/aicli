"""Step 5: Modular dimension analysis on answer units.

Each dimension is defined in prompts.yaml and analyzed independently.
Adding a new dimension requires only editing the YAML — no code changes.
"""
from concurrent.futures import ThreadPoolExecutor, as_completed
import json

from aicli.domains.analyze.database import AnalyzeDB
from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.analyze.config_loader import AnalyzeConfig


class DimensionAnalyzerService:
    """Run modular dimension analysis on answer units."""

    def __init__(self, provider: LMStudioProvider, config: AnalyzeConfig):
        self.provider = provider
        self.config = config

    def analyze_answer(self, answer: dict, dimension_name: str) -> dict:
        """Analyze one answer for one dimension.

        Args:
            answer: Answer dict from DB (must have 'raw_text').
            dimension_name: Name of the dimension (e.g., 'intro', 'outro').

        Returns:
            Parsed JSON result dict.
        """
        prompt_template = self.config.get_dimension_prompt(dimension_name)
        prompt = prompt_template.replace("{answer_text}", answer["raw_text"] or "")

        result = self.provider.complete_text_json(
            prompt=prompt,
            temperature=self.config.temperature,
            max_tokens=self.config.max_tokens,
            max_retries=self.config.max_retries,
            retry_backoff_base=self.config.retry_backoff_base,
        )

        return result

    def analyze_dimension(
        self,
        dimension_name: str,
        db: AnalyzeDB,
        workers: int = 4,
        progress=None,
        task_id=None,
    ) -> int:
        """Run a single dimension across all unanalyzed answers.

        Returns:
            Count of answers analyzed for this dimension.
        """
        answers = db.get_unanalyzed_answers(dimension_name)
        if not answers:
            return 0

        count = 0

        def _process_one(answer):
            result = self.analyze_answer(answer, dimension_name)
            db.insert_dimension_result(
                answer_id=answer["id"],
                dimension_name=dimension_name,
                result_json=json.dumps(result, ensure_ascii=False),
            )
            return answer["id"]

        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = {executor.submit(_process_one, a): a for a in answers}
            for future in as_completed(futures):
                answer = futures[future]
                try:
                    future.result()
                    count += 1
                    if progress and task_id is not None:
                        progress.advance(task_id)
                except Exception as e:
                    # Log error, store partial result, continue
                    db.log_processing(
                        answer.get("pdf_file"),
                        f"dimension_{dimension_name}",
                        "error",
                        str(e),
                    )
                    # Store error as result so we don't retry on resume
                    try:
                        db.insert_dimension_result(
                            answer_id=answer["id"],
                            dimension_name=dimension_name,
                            result_json=json.dumps({"error": str(e)}),
                        )
                    except Exception:
                        pass
                    if progress and task_id is not None:
                        progress.advance(task_id)

        return count

    def analyze_all_dimensions(
        self,
        db: AnalyzeDB,
        workers: int = 4,
        progress=None,
        task_id=None,
    ) -> dict[str, int]:
        """Run all enabled dimensions.

        Returns:
            {dimension_name: count_analyzed} map.
        """
        results = {}
        for dim_name in self.config.enabled_dimensions:
            count = self.analyze_dimension(
                dim_name, db, workers, progress, task_id
            )
            results[dim_name] = count
        return results
