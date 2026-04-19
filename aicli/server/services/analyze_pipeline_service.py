"""Service for orchestrating the UPSC Analyze pipeline."""
import time
import json
from pathlib import Path
from typing import List, Dict, Optional, Any

from aicli.server.constants.analyze_constants import (
    STEP_PDF_TO_IMAGES,
    STEP_OCR_TRANSCRIBE,
    STEP_PAGE_CLASSIFY,
    STEP_ANSWER_SEGMENT,
    STEP_DIMENSION_ANALYZE,
    STEP_CROSS_PDF_AGGREGATE,
    STEP_REPORT_GEN,
    RECOMMENDED_REASONING,
    TRANSCRIPTION_ERROR_PREFIX
)
from aicli.server.repositories.analyze_repository import AnalyzeRepository
from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.analyze.config_loader import AnalyzeConfig
from aicli.services.analyze.pdf_converter import PDFConverterService
from aicli.services.analyze.page_classifier import PageClassifierService
from aicli.services.analyze.transcriber import AnswerTranscriberService
from aicli.services.analyze.segmenter import AnswerSegmenterService
from aicli.services.analyze.dimension_analyzer import DimensionAnalyzerService
from aicli.services.analyze.aggregator import AggregationService
from aicli.services.analyze.report_generator import ReportGeneratorService

class AnalyzePipelineService:
    """Service to orchestrate the 7-step analysis pipeline."""

    def __init__(
        self, 
        repository: AnalyzeRepository, 
        provider: LMStudioProvider,
        config: AnalyzeConfig
    ):
        self._repo = repository
        self._provider = provider
        self._config = config
        self._db = repository._db # Reach into internal DB for legacy service compatibility

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
        log_callback: Optional[Any] = None
    ) -> float:
        """Execute the pipeline steps."""
        start_t = time.time()
        
        self._log(log_callback, "🚀 [SYSTEM] Industrializing UPSC Analyze Pipeline...")
        self._log(log_callback, f"⚙️ Config: model={llm_model}, workers={workers}, reasoning={allow_reasoning}")
        
        if target_steps:
            self._log(log_callback, f"🎯 Targeted execution: Steps {target_steps}")
        else:
            self._log(log_callback, "🔄 Full end-to-end execution selected.")
        
        # Helper to determine if a step should use "Deep Thinking"
        def should_think(step_id: int) -> bool:
            if not allow_reasoning:
                return False
            if step_reasoning:
                # Handle both string and int keys from JSON/Frontend
                val = step_reasoning.get(str(step_id)) or step_reasoning.get(step_id)
                if val is not None:
                    return bool(val)
            return RECOMMENDED_REASONING.get(step_id, False)

        # Step 1: PDF → Images
        if self._should_run(1, target_steps):
            self._log(log_callback, "Step 1: PDF → Images")
            converter = PDFConverterService()
            pdf_count, total_pages = converter.convert_all(data_dir, cache_dir, self._db, dpi)
            self._log(log_callback, f"Converted {pdf_count} PDF(s) → {total_pages} page images")

        # Step 2: OCR Transcription
        if self._should_run(2, target_steps):
            self._log(log_callback, "Step 2: OCR Transcription")
            transcriber = AnswerTranscriberService(self._provider, self._config)
            
            if target_page_id:
                page = self._db.get_page(target_page_id)
                if page:
                    try:
                        txt = transcriber.transcribe_page(page, allow_reasoning=should_think(2))
                        self._db.update_transcription(page["id"], txt)
                        self._log(log_callback, f"Transcribed page {target_page_id}")
                    except Exception as e:
                        self._db.update_transcription(page["id"], f"{TRANSCRIPTION_ERROR_PREFIX} {e}]")
                        self._log(log_callback, f"Error transcribing page {target_page_id}: {e}")
            else:
                untranscribed = self._db.get_untranscribed_pages()
                if untranscribed:
                    # In a real environment, we'd pass a specialized Progress object here
                    # For now, we use a mock if progress_callback is present
                    transcribed, errors = transcriber.transcribe_batch(
                        self._db, workers, progress_callback, None, 
                        allow_reasoning=should_think(2)
                    )
                    self._log(log_callback, f"Transcribed {transcribed} pages ({errors} errors)")
                else:
                    self._log(log_callback, "Step 2: No untranscribed pages. Skipping.")

        # Step 3: Page Classification
        if self._should_run(3, target_steps):
            self._log(log_callback, "Step 3: Page Classification")
            classifier = PageClassifierService(self._provider, self._config)
            
            if target_page_id:
                page = self._db.get_page(target_page_id)
                if page:
                    classifier.classify_page(page, allow_reasoning=should_think(3))
                    self._log(log_callback, f"Classified page {target_page_id}")
            else:
                unclassified = self._db.get_unclassified_pages()
                if unclassified:
                    classifier.classify_batch(self._db, workers, progress_callback, None, allow_reasoning=should_think(3))
                    self._log(log_callback, f"Classified {len(unclassified)} pages")
                else:
                    self._log(log_callback, "Step 3: No unclassified pages. Skipping.")

        # Step 4: Answer Segmentation
        if self._should_run(4, target_steps):
            self._log(log_callback, "Step 4: Answer Segmentation")
            segmenter = AnswerSegmenterService(self._provider, self._config)
            
            if target_page_id:
                page = self._db.get_page(target_page_id)
                if page:
                    pdf_filename = page["pdf_file"]
                    segmenter.segment_pdf(pdf_filename, self._db, allow_reasoning=should_think(4))
                    self._log(log_callback, f"Segmented {pdf_filename}")
            else:
                unsegmented = self._db.get_unsegmented_pdfs()
                if unsegmented:
                    segmenter.segment_all(self._db, progress_callback, None, allow_reasoning=should_think(4))
                    self._log(log_callback, f"Segmented {len(unsegmented)} PDFs")
                else:
                    self._log(log_callback, "Step 4: No unsegmented PDFs. Skipping.")

        # Step 5: Dimension Analysis
        if self._should_run(5, target_steps):
            self._log(log_callback, "Step 5: Dimension Analysis")
            analyzer = DimensionAnalyzerService(self._provider, self._config)
            enabled_dims = self._config.enabled_dimensions

            for dim_name in enabled_dims:
                unanalyzed = self._get_target_answers(dim_name, target_page_id)
                if unanalyzed:
                    self._log(log_callback, f"Analyzing {dim_name}: {len(unanalyzed)} answers")
                    # For targeted runs, we loop manually; otherwise, use batch
                    if target_page_id:
                        for ans in unanalyzed:
                            analyzer.analyze_answer(ans, dim_name, self._db, allow_reasoning=should_think(5))
                    else:
                        analyzer.analyze_dimension(dim_name, self._db, workers, progress_callback, None, allow_reasoning=should_think(5))
                    self._log(log_callback, f"Completed {dim_name}")

        # Step 6: Aggregation
        if self._should_run(6, target_steps):
            self._log(log_callback, "Step 6: Cross-PDF Aggregation")
            aggregator = AggregationService(self._provider, self._config)
            agg_count = aggregator.aggregate_all(self._db, progress_callback, None, allow_reasoning=should_think(6))
            self._log(log_callback, f"Aggregated {agg_count} dimensions")

        # Step 7: Report Generation
        if self._should_run(7, target_steps):
            self._log(log_callback, "Step 7: Report Generation")
            reporter = ReportGeneratorService()
            md_path, _ = reporter.generate(self._db, data_dir)
            self._log(log_callback, f"Report generated: {md_path}")

        return time.time() - start_t

    def _should_run(self, step_id: int, target_steps: Optional[List[int]]) -> bool:
        """Check if a step should be executed based on the target selection."""
        # Treat None or empty list as full pipeline (all steps)
        return not target_steps or step_id in target_steps

    def _log(self, callback: Optional[Any], message: str):
        if callback:
            callback(message)

    def _get_target_answers(self, dim_name: str, target_page_id: Optional[int]) -> List[Dict]:
        """Fetch answers for analysis, optionally filtered by page ID."""
        if not target_page_id:
            return self._db.get_unanalyzed_answers(dim_name)
        
        # Targeted page logic
        all_un = self._db.get_unanalyzed_answers(dim_name)
        unanalyzed = []
        for ans in all_un:
            try:
                ids = json.loads(ans["page_ids"])
                if target_page_id in ids or str(target_page_id) in ids:
                    unanalyzed.append(ans)
            except: pass
        
        if not unanalyzed:
            # Force rerun for targeted page
            with self._db._get_conn() as conn:
                rows = conn.execute("SELECT * FROM answers").fetchall()
                for r in rows:
                    ans = dict(r)
                    try:
                        ids = json.loads(ans["page_ids"])
                        if target_page_id in ids or str(target_page_id) in ids:
                            unanalyzed.append(ans)
                    except: pass
        return unanalyzed
