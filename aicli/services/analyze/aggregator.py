"""Step 6: Cross-PDF aggregation of dimension patterns.

For each dimension, collects all per-answer results and sends them to
the LM for pattern identification across candidates.
"""

import json

from aicli.domains.analyze.database import AnalyzeDB
from aicli.core.interfaces import ImageVisionProvider
from aicli.services.analyze.config_loader import AnalyzeConfig


class AggregationService:
    """Aggregate dimension results across all PDFs to find reusable patterns."""

    def __init__(self, provider: ImageVisionProvider, config: AnalyzeConfig):
        self.provider = provider
        self.config = config

    def aggregate_dimension(
        self, dimension_name: str, db: AnalyzeDB, allow_reasoning: bool = True
    ) -> dict:
        """Aggregate all results for one dimension.

        Returns:
            Aggregation JSON dict.
        """
        results = db.get_dimension_results(dimension_name)
        if not results:
            return {}

        # Filter out error results
        valid_results = []
        for r in results:
            try:
                parsed = json.loads(r["result_json"])
                if "error" not in parsed:
                    valid_results.append(
                        {
                            "candidate": r.get("candidate_name", "unknown"),
                            "pdf_file": r.get("pdf_file", ""),
                            "question_number": r.get("question_number", ""),
                            "question_directive": r.get("question_directive", ""),
                            "analysis": parsed,
                        }
                    )
            except (json.JSONDecodeError, TypeError):
                continue

        if not valid_results:
            return {}

        # Count unique candidates
        candidates = set()
        for r in valid_results:
            candidates.add(r.get("candidate") or r.get("pdf_file") or "unknown")

        # Build dimension data string
        dimension_data = json.dumps(valid_results, indent=2, ensure_ascii=False)

        # Build the aggregation prompt
        prompt_template = self.config.aggregation_prompt_template
        prompt = prompt_template.format(
            count=len(valid_results),
            candidates=len(candidates),
            dimension_name=dimension_name,
            dimension_data=dimension_data,
        )

        # If the data is too large, chunk it
        # Rough estimate: ~4 chars per token, limit to ~6000 tokens for context
        max_prompt_chars = 24000
        if len(prompt) > max_prompt_chars:
            aggregation = self._chunked_aggregation(
                dimension_name,
                valid_results,
                candidates,
                db,
                allow_reasoning=allow_reasoning,
            )
        else:
            aggregation = self.provider.complete_text_json(
                prompt=prompt,
                temperature=self.config.temperature,
                max_tokens=8192,
                max_retries=self.config.max_retries,
                retry_backoff_base=self.config.retry_backoff_base,
                allow_reasoning=allow_reasoning,
            )

        # Store in DB
        db.insert_aggregation(
            dimension_name=dimension_name,
            aggregation_json=json.dumps(aggregation, ensure_ascii=False),
            answer_count=len(valid_results),
        )
        db.log_processing(None, f"aggregation_{dimension_name}", "done")

        return aggregation

    def aggregate_all(
        self, db: AnalyzeDB, progress=None, task_id=None, allow_reasoning: bool = True
    ) -> int:
        """Run aggregation for all enabled dimensions.

        Returns:
            Count of dimensions aggregated.
        """
        count = 0
        for dim_name in self.config.enabled_dimensions:
            results = db.get_dimension_results(dim_name)
            if results:
                self.aggregate_dimension(dim_name, db, allow_reasoning=allow_reasoning)
                count += 1
                if progress and task_id is not None:
                    progress.advance(task_id)

        return count

    def _chunked_aggregation(
        self,
        dimension_name: str,
        all_results: list[dict],
        candidates: set,
        db: AnalyzeDB,
        allow_reasoning: bool = True,
    ) -> dict:
        """Handle large datasets by aggregating in chunks and then merging.

        Splits results into chunks, summarizes each, then combines summaries.
        """
        chunk_size = 50  # ~50 answers per chunk
        chunks = [
            all_results[i : i + chunk_size]
            for i in range(0, len(all_results), chunk_size)
        ]

        chunk_summaries = []
        for chunk in chunks:
            chunk_data = json.dumps(chunk, indent=2, ensure_ascii=False)
            prompt = self.config.aggregation_prompt_template.format(
                count=len(chunk),
                candidates=len(candidates),
                dimension_name=dimension_name,
                dimension_data=chunk_data,
            )

            try:
                summary = self.provider.complete_text_json(
                    prompt=prompt,
                    temperature=self.config.temperature,
                    max_tokens=8192,
                    max_retries=self.config.max_retries,
                    retry_backoff_base=self.config.retry_backoff_base,
                    allow_reasoning=allow_reasoning,
                )
                chunk_summaries.append(summary)
            except Exception:
                continue

        if len(chunk_summaries) == 1:
            return chunk_summaries[0]

        # Merge chunk summaries
        merge_data = json.dumps(chunk_summaries, indent=2, ensure_ascii=False)
        merge_prompt = (
            f"You previously analyzed the [{dimension_name}] dimension in {len(chunks)} batches.\n"
            f"Below are the aggregation results from each batch.\n\n"
            f"{merge_data}\n\n"
            f"Now merge these into a single unified aggregation. "
            f"Combine duplicate patterns, sum frequencies, keep the best 3+ examples per pattern.\n"
            f"Total answers across all batches: {len(all_results)}\n"
            f"Total unique candidates: {len(candidates)}\n\n"
            f"Return the same JSON format as the individual aggregations."
        )

        merged = self.provider.complete_text_json(
            prompt=merge_prompt,
            temperature=self.config.temperature,
            max_tokens=8192,
            max_retries=self.config.max_retries,
            retry_backoff_base=self.config.retry_backoff_base,
            allow_reasoning=allow_reasoning,
        )

        return merged
