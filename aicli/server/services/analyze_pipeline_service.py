"""Service for orchestrating the UPSC Analyze pipeline."""

import json
import time
from pathlib import Path
from typing import List, Dict, Optional, Any, Callable

from aicli.server.constants.analyze_constants import TRANSCRIPTION_ERROR_PREFIX
from aicli.server.repositories.analyze_repository import AnalyzeRepository
from aicli.server.services.reasoning_resolver import ReasoningResolver
from aicli.providers.ollama import OllamaProvider
from aicli.services.analyze.config_loader import AnalyzeConfig
from aicli.services.analyze.pdf_converter import PDFConverterService
from aicli.services.analyze.page_classifier import PageClassifierService
from aicli.services.analyze.transcriber import AnswerTranscriberService
from aicli.services.analyze.segmenter import AnswerSegmenterService
from aicli.services.analyze.dimension_analyzer import DimensionAnalyzerService
from aicli.services.analyze.aggregator import AggregationService
from aicli.services.analyze.report_generator import ReportGeneratorService


class AnalyzePipelineService:
    """Service to orchestrate the 7-step analysis pipeline.

    Each step is a small, focused method. The service delegates DB access
    to the repository and reasoning decisions to the ReasoningResolver.
    """

    def __init__(
        self,
        repository: AnalyzeRepository,
        provider: LMStudioProvider,
        config: AnalyzeConfig,
    ) -> None:
        self._repo = repository
        self._provider = provider
        self._config = config

    def run_full_pipeline(
        self,
        data_dir: Path,
        cache_dir: Path,
        workers: int,
        dpi: int,
        llm_model: str,
        allow_reasoning: bool = True,
        target_steps: Optional[List[int]] = None,
        step_reasoning: Optional[Dict[str, bool]] = None,
        target_page_id: Optional[int] = None,
        progress_callback: Optional[Any] = None,
        log_callback: Optional[Callable[[str], None]] = None,
        abort_event: Optional[Any] = None,
    ) -> float:
        """Execute the pipeline steps."""
        start_t = time.time()
        resolver = ReasoningResolver(allow_reasoning, step_reasoning)
        db = self._repo._db  # Domain DB needed by legacy service constructors

        self._log(log_callback, "🚀 [SYSTEM] Industrializing UPSC Analyze Pipeline...")
        self._log(
            log_callback,
            f"⚙️ Config: model={llm_model}, workers={workers}, reasoning={allow_reasoning}",
        )
        self._log_target_info(log_callback, target_steps)

        if llm_model:
            from aicli.config import resolve_dynamic_model, config as aicli_config

            self._log(log_callback, f"🔄 Loading/Verifying model: {llm_model}...")
            try:
                resolved = resolve_dynamic_model(llm_model)
                aicli_config.model_name = resolved
                self._log(log_callback, f"✔ Model ready: {resolved}")
            except Exception as e:
                self._log(
                    log_callback, f"⚠️ Warning: Model loading may have failed ({e})"
                )

        ctx = _PipelineContext(
            data_dir=data_dir,
            cache_dir=cache_dir,
            workers=workers,
            dpi=dpi,
            db=db,
            resolver=resolver,
            target_page_id=target_page_id,
            progress=progress_callback,
            log_cb=log_callback,
            abort_event=abort_event,
        )

        if self._should_run(1, target_steps):
            self._step_pdf_to_images(ctx)
        if self._should_run(2, target_steps):
            self._step_ocr_transcribe(ctx)
        if self._should_run(3, target_steps):
            self._step_page_classify(ctx)
        if self._should_run(4, target_steps):
            self._step_answer_segment(ctx)
        if self._should_run(5, target_steps):
            self._step_dimension_analyze(ctx)
        if self._should_run(6, target_steps):
            self._step_aggregate(ctx)
        if self._should_run(7, target_steps):
            self._step_report_generate(ctx)

        return time.time() - start_t

    # ── Step Methods (each ≤ 15 lines) ──────────────────────────────

    def _step_pdf_to_images(self, ctx: "_PipelineContext") -> None:
        self._log(ctx.log_cb, "Step 1: PDF → Images")
        converter = PDFConverterService()
        pdf_count, total_pages = converter.convert_all(
            ctx.data_dir, ctx.cache_dir, ctx.db, ctx.dpi
        )
        self._log(
            ctx.log_cb, f"Converted {pdf_count} PDF(s) → {total_pages} page images"
        )

    def _step_ocr_transcribe(self, ctx: "_PipelineContext") -> None:
        self._log(ctx.log_cb, "Step 2: OCR Transcription")
        transcriber = AnswerTranscriberService(self._provider, self._config)
        if ctx.target_page_id:
            self._transcribe_single_page(transcriber, ctx)
        else:
            self._transcribe_batch(transcriber, ctx)

    def _transcribe_batch(
        self, transcriber: AnswerTranscriberService, ctx: "_PipelineContext"
    ) -> None:
        think = ctx.resolver.should_think(2)
        success, err = transcriber.transcribe_batch(
            ctx.db,
            workers=ctx.workers,
            progress=ctx.progress,
            allow_reasoning=think,
            abort_event=ctx.abort_event,
        )
        self._log(
            ctx.log_cb, f"Transcription completed. Success: {success}, Errors: {err}"
        )

    def _transcribe_single_page(
        self, transcriber: AnswerTranscriberService, ctx: "_PipelineContext"
    ) -> None:
        page_raw = ctx.db._get_by_id("pages", ctx.target_page_id)
        if page_raw and not page_raw.get("transcription_text"):
            page_row = ctx.db._page_tuple_to_dict(page_raw)
            think = ctx.resolver.should_think(2)
            transcription = transcriber.transcribe_page(
                page_row, allow_reasoning=think, abort_event=ctx.abort_event
            )
            ctx.db.update_transcription(page_row["id"], transcription)
        self._log(
            ctx.log_cb,
            f"Transcription completed for explicit page target (ID={ctx.target_page_id})",
        )

    def _step_page_classify(self, ctx: "_PipelineContext") -> None:
        self._log(ctx.log_cb, "Step 3: Page Classification")
        classifier = PageClassifierService(self._provider, self._config)
        if ctx.target_page_id:
            self._classify_single_page(classifier, ctx)
        else:
            self._classify_batch(classifier, ctx)

    def _step_answer_segment(self, ctx: "_PipelineContext") -> None:
        self._log(ctx.log_cb, "Step 4: Answer Segmentation")
        segmenter = AnswerSegmenterService(self._provider, self._config)
        if ctx.target_page_id:
            self._segment_single_pdf(segmenter, ctx)
        else:
            self._segment_batch(segmenter, ctx)

    def _step_dimension_analyze(self, ctx: "_PipelineContext") -> None:
        self._log(ctx.log_cb, "Step 5: Dimension Analysis")
        analyzer = DimensionAnalyzerService(self._provider, self._config)
        for dim_name in self._config.enabled_dimensions:
            self._analyze_dimension(analyzer, dim_name, ctx)

    def _step_aggregate(self, ctx: "_PipelineContext") -> None:
        self._log(ctx.log_cb, "Step 6: Cross-PDF Aggregation")
        aggregator = AggregationService(self._provider, self._config)
        think = ctx.resolver.should_think(6)
        agg_count = aggregator.aggregate_all(
            ctx.db, ctx.progress, None, allow_reasoning=think
        )
        self._log(ctx.log_cb, f"Aggregated {agg_count} dimensions")

    def _step_report_generate(self, ctx: "_PipelineContext") -> None:
        self._log(ctx.log_cb, "Step 7: Report Generation")
        reporter = ReportGeneratorService()
        md_path, _ = reporter.generate(ctx.db, ctx.data_dir)
        self._log(ctx.log_cb, f"Report generated: {md_path}")

    # ── Sub-step Helpers ────────────────────────────────────────────

    def _classify_single_page(
        self, classifier: PageClassifierService, ctx: "_PipelineContext"
    ) -> None:
        page = ctx.db.get_page(ctx.target_page_id)
        if page:
            classifier.classify_page(page, allow_reasoning=ctx.resolver.should_think(3))
            self._log(ctx.log_cb, f"Classified page {ctx.target_page_id}")

    def _classify_batch(
        self, classifier: PageClassifierService, ctx: "_PipelineContext"
    ) -> None:
        unclassified = ctx.db.get_unclassified_pages()
        if not unclassified:
            self._log(ctx.log_cb, "Step 3: No unclassified pages. Skipping.")
            return
        think = ctx.resolver.should_think(3)
        classifier.classify_batch(
            ctx.db, ctx.workers, ctx.progress, None, allow_reasoning=think
        )
        self._log(ctx.log_cb, f"Classified {len(unclassified)} pages")

    def _segment_single_pdf(
        self, segmenter: AnswerSegmenterService, ctx: "_PipelineContext"
    ) -> None:
        page = ctx.db.get_page(ctx.target_page_id)
        if page:
            think = ctx.resolver.should_think(4)
            segmenter.segment_pdf(page["pdf_file"], ctx.db, allow_reasoning=think)
            self._log(ctx.log_cb, f"Segmented {page['pdf_file']}")

    def _segment_batch(
        self, segmenter: AnswerSegmenterService, ctx: "_PipelineContext"
    ) -> None:
        unsegmented = ctx.db.get_unsegmented_pdfs()
        if not unsegmented:
            self._log(ctx.log_cb, "Step 4: No unsegmented PDFs. Skipping.")
            return
        think = ctx.resolver.should_think(4)
        segmenter.segment_all(ctx.db, ctx.progress, None, allow_reasoning=think)
        self._log(ctx.log_cb, f"Segmented {len(unsegmented)} PDFs")

    def _analyze_dimension(
        self, analyzer: DimensionAnalyzerService, dim_name: str, ctx: "_PipelineContext"
    ) -> None:
        unanalyzed = self._get_target_answers(dim_name, ctx)
        if not unanalyzed:
            return
        self._log(ctx.log_cb, f"Analyzing {dim_name}: {len(unanalyzed)} answers")
        think = ctx.resolver.should_think(5)
        if ctx.target_page_id:
            for ans in unanalyzed:
                analyzer.analyze_answer(ans, dim_name, ctx.db, allow_reasoning=think)
        else:
            analyzer.analyze_dimension(
                dim_name, ctx.db, ctx.workers, ctx.progress, None, allow_reasoning=think
            )
        self._log(ctx.log_cb, f"Completed {dim_name}")

    # ── Utility ─────────────────────────────────────────────────────

    def _get_target_answers(self, dim_name: str, ctx: "_PipelineContext") -> List[Dict]:
        """Fetch answers for analysis, optionally filtered by page ID."""
        if not ctx.target_page_id:
            return ctx.db.get_unanalyzed_answers(dim_name)

        # Targeted page: first try unanalyzed, then fall back to all answers
        unanalyzed = self._filter_answers_by_page(
            ctx.db.get_unanalyzed_answers(dim_name), ctx.target_page_id
        )
        if unanalyzed:
            return unanalyzed
        # Force re-run for targeted page
        return self._repo.get_answers_for_page(ctx.target_page_id)

    @staticmethod
    def _filter_answers_by_page(answers: List[Dict], page_id: int) -> List[Dict]:
        """Filter answers to those referencing a specific page."""
        result = []
        for ans in answers:
            try:
                ids = json.loads(ans["page_ids"])
                if page_id in ids or str(page_id) in ids:
                    result.append(ans)
            except (json.JSONDecodeError, KeyError):
                continue
        return result

    @staticmethod
    def _should_run(step_id: int, target_steps: Optional[List[int]]) -> bool:
        """Check if a step should be executed based on the target selection."""
        return not target_steps or step_id in target_steps

    @staticmethod
    def _log(callback: Optional[Callable], message: str) -> None:
        if callback:
            callback(message)

    @staticmethod
    def _log_target_info(
        callback: Optional[Callable], target_steps: Optional[List[int]]
    ) -> None:
        if target_steps:
            AnalyzePipelineService._log(
                callback, f"🎯 Targeted execution: Steps {target_steps}"
            )
        else:
            AnalyzePipelineService._log(
                callback, "🔄 Full end-to-end execution selected."
            )


class _PipelineContext:
    """Lightweight data bag passed to each step method, avoiding long parameter lists."""

    __slots__ = (
        "data_dir",
        "cache_dir",
        "workers",
        "dpi",
        "db",
        "resolver",
        "target_page_id",
        "progress",
        "log_cb",
        "abort_event",
    )

    def __init__(
        self,
        data_dir: Path,
        cache_dir: Path,
        workers: int,
        dpi: int,
        db: Any,
        resolver: ReasoningResolver,
        target_page_id: Optional[int],
        progress: Optional[Any],
        log_cb: Optional[Callable],
        abort_event: Optional[Any] = None,
    ) -> None:
        self.data_dir = data_dir
        self.cache_dir = cache_dir
        self.workers = workers
        self.dpi = dpi
        self.db = db
        self.resolver = resolver
        self.target_page_id = target_page_id
        self.progress = progress
        self.log_cb = log_cb
        self.abort_event = abort_event
