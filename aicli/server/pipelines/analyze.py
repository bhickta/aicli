"""CLI commands for the UPSC topper answer sheet analysis pipeline.

Commands:
    aicli analyze pdfs ./data/       — Run full pipeline on all PDFs
    aicli analyze pdf ./data/001.pdf — Run on single PDF
    aicli analyze status             — Show processing progress
    aicli analyze dimension intro    — Run/re-run single dimension
    aicli analyze aggregate          — Run step 6 aggregation
    aicli analyze report             — Generate final markdown report
    aicli analyze reset --step 3     — Reset from step 3 onwards
"""
import time
from pathlib import Path

import typer
from rich.progress import (
    Progress,
    SpinnerColumn,
    TextColumn,
    BarColumn,
    TaskProgressColumn,
    TimeElapsedColumn,
    TimeRemainingColumn,
)
from rich.table import Table

from aicli.cli.tui import print_header, print_success, print_error, console
from aicli.domains.analyze.database import AnalyzeDB
from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.analyze.config_loader import AnalyzeConfig
from aicli.services.analyze.pdf_converter import PDFConverterService
from aicli.services.analyze.page_classifier import PageClassifierService
from aicli.services.analyze.transcriber import AnswerTranscriberService
from aicli.services.analyze.segmenter import AnswerSegmenterService
from aicli.services.analyze.dimension_analyzer import DimensionAnalyzerService
from aicli.services.analyze.aggregator import AggregationService
from aicli.services.analyze.report_generator import ReportGeneratorService

app = typer.Typer(help="UPSC topper answer sheet analysis pipeline.")

# ---------------------------------------------------------------------------
# Shared helpers
# ---------------------------------------------------------------------------

def _get_data_dir(path: Path) -> Path:
    """Resolve the data directory from a file or directory path."""
    return path if path.is_dir() else path.parent


def _get_db(data_dir: Path) -> AnalyzeDB:
    """Open (or create) the analyze database in the data directory."""
    db_path = data_dir / "analyze.db"
    return AnalyzeDB(db_path)


def _get_cache_dir(data_dir: Path) -> Path:
    """Get the image cache directory."""
    cache = data_dir / ".analyze_cache" / "images"
    cache.mkdir(parents=True, exist_ok=True)
    return cache


def _make_progress() -> Progress:
    """Create a consistent Rich progress bar."""
    return Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TaskProgressColumn(),
        TimeElapsedColumn(),
        TextColumn("•"),
        TimeRemainingColumn(),
        console=console,
    )


def _ensure_model(llm_model: str | None = None):
    """Load preferred model into LM Studio if --llm is specified."""
    if not llm_model:
        return
    from aicli.config import resolve_dynamic_model, config as aicli_config
    console.print(f"[cyan]Loading model matching '{llm_model}'...[/cyan]")
    resolved = resolve_dynamic_model(llm_model)
    aicli_config.model_name = resolved
    console.print(f"[green]✔ Model ready: {resolved}[/green]")

def _run_full_pipeline(
    data_dir: Path,
    db: AnalyzeDB,
    workers: int,
    dpi: int,
    pdf_files: list[Path] | None = None,
    llm_model: str = "gemma-4-26b-a4b",
    allow_reasoning: bool = True,
    target_steps: list[int] | None = None,
    step_reasoning: dict[int, bool] | None = None,
    target_page_id: int | None = None,
) -> float:
    """Execute the full 7-step pipeline."""
    cache_dir = _get_cache_dir(data_dir)
    cfg = AnalyzeConfig()
    _ensure_model(llm_model)
    provider = LMStudioProvider()

    start_t = time.time()

    # Defensive type-casting for target_steps
    if target_steps is not None:
        target_steps = [int(s) for s in target_steps]

    # Smart Reasoning Logic: Default to OFF for extraction, ON for analysis
    # User can override per-step, OR disable reasoning globally by turning OFF master switch.
    RECOM_REASONING = {2: False, 3: False, 4: False, 5: True, 6: True, 7: True}
    
    def _step_think(step_id: int) -> bool:
        if not allow_reasoning:
            return False # Global Master Switch is OFF
        if step_reasoning and str(step_id) in step_reasoning:
            return bool(step_reasoning[str(step_id)])
        if step_reasoning and step_id in step_reasoning:
            return bool(step_reasoning[step_id])
        return RECOM_REASONING.get(step_id, False)

    # ------------------------------------------------------------------
    # Step 1: PDF → Images
    # ------------------------------------------------------------------
    if target_steps is None or 1 in target_steps:
        console.print("\n[bold magenta]━━━ Step 1: PDF → Images ━━━[/bold magenta]")
        converter = PDFConverterService()

        if pdf_files:
            total_pages = 0
            for pf in pdf_files:
                n = converter.convert(pf, cache_dir, db, dpi)
                total_pages += n
            pdf_count = len(pdf_files)
        else:
            pdf_count, total_pages = converter.convert_all(data_dir, cache_dir, db, dpi)

        if total_pages > 0:
            print_success(f"Converted {pdf_count} PDF(s) → {total_pages} page images")
        else:
            console.print("[dim]Step 1: All PDFs already converted. Skipping.[/dim]")
    elif target_steps is not None and 1 not in target_steps:
        pass # Explicitly not requested

    # ------------------------------------------------------------------
    # Step 2: OCR Transcription (Vision Model — all pages)
    # ------------------------------------------------------------------
    if target_steps is None or 2 in target_steps:
        console.print("\n[bold magenta]━━━ Step 2: OCR Transcription ━━━[/bold magenta]")
        transcriber = AnswerTranscriberService(provider, cfg)
        
        if target_page_id:
            page = db.get_page(target_page_id)
            untranscribed = [page] if page else []
        else:
            untranscribed = db.get_untranscribed_pages()

        if untranscribed:
            with _make_progress() as progress:
                # Use a larger task description if targeting
                desc = f"Transcribing page {target_page_id}..." if target_page_id else "Transcribing pages..."
                task = progress.add_task(desc, total=len(untranscribed))
                # If targeting, we must pass the specific list because transcribe_batch usually fetches its own
                if target_page_id:
                    transcribed = 0
                    errors = 0
                    for p in untranscribed:
                        try:
                            # Update directly
                            txt = transcriber.transcribe_page(p, allow_reasoning=_step_think(2))
                            db.update_transcription(p["id"], txt)
                            transcribed += 1
                        except Exception as e:
                            errors += 1
                            db.update_transcription(p["id"], f"[TRANSCRIPTION_ERROR: {e}]")
                        progress.advance(task)
                    tr_errors = errors
                else:
                    transcribed, tr_errors = transcriber.transcribe_batch(
                        db, workers, progress, task, 
                        allow_reasoning=allow_reasoning
                    )
            
            if tr_errors:
                print_error(f"Transcription: {tr_errors} errors", ValueError(""))
            print_success(f"Transcribed {transcribed} pages")
        else:
            console.print("[dim]Step 2: No untranscribed pages. Skipping.[/dim]")

    # ------------------------------------------------------------------
    # Step 3: Page Classification (Text-only — fast)
    # ------------------------------------------------------------------
    if target_steps is None or 3 in target_steps:
        console.print("\n[bold magenta]━━━ Step 3: Page Classification ━━━[/bold magenta]")
        classifier = PageClassifierService(provider, cfg)
        
        if target_page_id:
            page = db.get_page(target_page_id)
            unclassified = [page] if page else []
            if page:
                classifier.classify_page(page, allow_reasoning=_step_think(3))
                print_success(f"Classified page {target_page_id}")
        else:
            unclassified = db.get_unclassified_pages()
            if unclassified:
                with _make_progress() as progress:
                    task = progress.add_task("Classifying pages...", total=len(unclassified))
                    classifier.classify_all(db, progress, task, allow_reasoning=_step_think(3))
            else:
                console.print("[dim]Step 3: No unclassified pages. Skipping.[/dim]")

    # ------------------------------------------------------------------
    # Step 4: Answer Segmentation
    # ------------------------------------------------------------------
    if target_steps is None or 4 in target_steps:
        console.print("\n[bold magenta]━━━ Step 4: Answer Segmentation ━━━[/bold magenta]")
        segmenter = AnswerSegmenterService(provider, cfg)
        
        if target_page_id:
            page = db.get_page(target_page_id)
            if page:
                pdf_path = Path(data_dir / page["pdf_file"])
                segmenter.segment_pdf(pdf_path, db, allow_reasoning=_step_think(4))
                print_success(f"Segmented {pdf_path.name}")
        else:
            unsegmented = db.get_unsegmented_pdfs()
            if unsegmented:
                with _make_progress() as progress:
                    task = progress.add_task("Segmenting PDFs...", total=len(unsegmented))
                    segmenter.segment_all(db, progress, task, allow_reasoning=_step_think(4))
            else:
                console.print("[dim]No unsegmented PDFs. Skipping.[/dim]")

    # ------------------------------------------------------------------
    # Step 5: Dimension Analysis
    # ------------------------------------------------------------------
    if target_steps is None or 5 in target_steps:
        console.print("\n[bold magenta]━━━ Step 5: Dimension Analysis ━━━[/bold magenta]")
        analyzer = DimensionAnalyzerService(provider, cfg)
        enabled_dims = cfg.enabled_dimensions

        for dim_name in enabled_dims:
            if target_page_id:
                # Find answers associated with this page
                # We can reuse part of get_unanalyzed_answers logic or filter it
                all_un = db.get_unanalyzed_answers(dim_name)
                # Filter client-side for simplicity since we have targeted the run
                unanalyzed = []
                for ans in all_un:
                    try:
                        ids = json.loads(ans["page_ids"])
                        if target_page_id in ids or str(target_page_id) in ids:
                            unanalyzed.append(ans)
                    except: pass
                # FORCE RERUN: If not in "unanalyzed", maybe it was already analyzed?
                # For target_page_id, we should probably allow re-running even if analyzed.
                if not unanalyzed:
                    # Get ALL answers for that page
                    rows = db._get_conn().execute("SELECT * FROM answers").fetchall()
                    for r in rows:
                        ans = dict(r)
                        try:
                            ids = json.loads(ans["page_ids"])
                            if target_page_id in ids or str(target_page_id) in ids:
                                unanalyzed.append(ans)
                        except: pass
            else:
                unanalyzed = db.get_unanalyzed_answers(dim_name)

            if unanalyzed:
                console.print(f"  [cyan]→ {dim_name}[/cyan]: {len(unanalyzed)} answers to analyze")
                with _make_progress() as progress:
                    task = progress.add_task(f"Analyzing [{dim_name}]...", total=len(unanalyzed))
                    if target_page_id:
                        count = 0
                        for ans in unanalyzed:
                            analyzer.analyze_answer(ans, dim_name, db, allow_reasoning=_step_think(5))
                            count += 1
                            progress.advance(task)
                    else:
                        count = analyzer.analyze_dimension(dim_name, db, workers, progress, task, allow_reasoning=_step_think(5))
                print_success(f"  {dim_name}: {count} answers analyzed")
            else:
                if not target_page_id:
                    console.print(f"  [dim]{dim_name}: already complete. Skipping.[/dim]")

    # ------------------------------------------------------------------
    # Step 6: Aggregation
    # ------------------------------------------------------------------
    if target_steps is None or 6 in target_steps:
        console.print("\n[bold magenta]━━━ Step 6: Cross-PDF Aggregation ━━━[/bold magenta]")
        aggregator = AggregationService(provider, cfg)

        with _make_progress() as progress:
            task = progress.add_task("Aggregating patterns...", total=len(enabled_dims))
            agg_count = aggregator.aggregate_all(db, progress, task, allow_reasoning=_step_think(6))
        print_success(f"Aggregated {agg_count} dimensions")

    # ------------------------------------------------------------------
    # Step 7: Report Generation
    # ------------------------------------------------------------------
    if target_steps is None or 7 in target_steps:
        console.print("\n[bold magenta]━━━ Step 7: Report Generation ━━━[/bold magenta]")
        reporter = ReportGeneratorService()
        md_path, json_path = reporter.generate(db, data_dir)
        print_success(f"Report: {md_path}")
        print_success(f"JSON:   {json_path}")


    elapsed = time.time() - start_t
    console.print(f"\n[bold green]✔ Full pipeline completed in {elapsed:.1f}s[/bold green]")


# ---------------------------------------------------------------------------
# CLI Commands
# ---------------------------------------------------------------------------

@app.command("pdfs")
def analyze_pdfs(
    data_dir: Path = typer.Argument(
        ...,
        exists=True,
        file_okay=False,
        dir_okay=True,
        help="Directory containing PDF files.",
    ),
    workers: int = typer.Option(4, "--workers", "-w", help="Number of concurrent workers."),
    dpi: int = typer.Option(200, "--dpi", help="PDF rendering DPI (200 recommended for handwriting)."),
    llm_model: str = typer.Option(None, "--llm", help="Model to load (e.g. 'gemma-4-26b-a4b')."),
):
    """Run full pipeline on all PDFs in a directory."""
    pdf_files = sorted(data_dir.glob("*.pdf"))
    if not pdf_files:
        print_error("No PDF files found", ValueError(f"No .pdf files in {data_dir}"))
        raise typer.Exit(1)

    print_header(f"UPSC Analyze — {len(pdf_files)} PDFs")
    console.print(f"[dim]Workers: {workers} | DPI: {dpi} | DB: {data_dir / 'analyze.db'}[/dim]")

    db = _get_db(data_dir)
    try:
        _run_full_pipeline(data_dir, db, workers, dpi, llm_model=llm_model)
    finally:
        db.close()


@app.command("pdf")
def analyze_pdf(
    pdf_path: Path = typer.Argument(
        ...,
        exists=True,
        file_okay=True,
        dir_okay=False,
        help="Path to a single PDF file.",
    ),
    workers: int = typer.Option(4, "--workers", "-w", help="Number of concurrent workers."),
    dpi: int = typer.Option(200, "--dpi", help="PDF rendering DPI."),
    llm_model: str = typer.Option(None, "--llm", help="Model to load (e.g. 'gemma-4-26b-a4b')."),
):
    """Run full pipeline on a single PDF."""
    data_dir = _get_data_dir(pdf_path)

    print_header(f"UPSC Analyze — {pdf_path.name}")
    console.print(f"[dim]Workers: {workers} | DPI: {dpi}[/dim]")

    db = _get_db(data_dir)
    try:
        _run_full_pipeline(data_dir, db, workers, dpi, pdf_files=[pdf_path], llm_model=llm_model)
    finally:
        db.close()


@app.command("status")
def show_status(
    data_dir: Path = typer.Argument(
        ".",
        exists=True,
        file_okay=False,
        dir_okay=True,
        help="Data directory (default: current directory).",
    ),
):
    """Show processing progress across all steps."""
    db_path = data_dir / "analyze.db"
    if not db_path.exists():
        print_error("No database found", ValueError(f"No analyze.db in {data_dir}"))
        raise typer.Exit(1)

    db = _get_db(data_dir)
    try:
        status = db.get_processing_status()
    finally:
        db.close()

    print_header("UPSC Analyze — Pipeline Status")

    # Overview table
    table = Table(title="Pipeline Overview", show_header=True, header_style="bold cyan")
    table.add_column("Metric", style="white")
    table.add_column("Count", style="green", justify="right")

    table.add_row("Total PDFs", str(status.get("total_pdfs", 0)))
    table.add_row("Total Pages", str(status.get("total_pages", 0)))
    table.add_row("Classified Pages", str(status.get("classified_pages", 0)))
    table.add_row("Transcribed Pages", str(status.get("transcribed_pages", 0)))
    table.add_row("Answer Units", str(status.get("total_answers", 0)))
    console.print(table)

    # Dimensions table
    dims = status.get("dimensions", {})
    if dims:
        dim_table = Table(title="Dimension Analysis", show_header=True, header_style="bold cyan")
        dim_table.add_column("Dimension", style="white")
        dim_table.add_column("Answers Analyzed", style="green", justify="right")
        dim_table.add_column("Aggregated", style="yellow", justify="center")

        aggs = status.get("aggregations", {})
        for dim_name, count in dims.items():
            agg_status = "✔" if dim_name in aggs else "—"
            dim_table.add_row(dim_name, str(count), agg_status)

        console.print(dim_table)

    # Errors table
    errors = status.get("errors", {})
    if errors:
        err_table = Table(title="Errors", show_header=True, header_style="bold red")
        err_table.add_column("Step", style="white")
        err_table.add_column("Count", style="red", justify="right")
        for step, count in errors.items():
            err_table.add_row(step, str(count))
        console.print(err_table)


@app.command("dimension")
def run_dimension(
    name: str = typer.Argument(..., help="Dimension name (e.g., intro, outro, formatting)."),
    data_dir: Path = typer.Option(
        ".",
        "--data-dir", "-d",
        exists=True,
        file_okay=False,
        dir_okay=True,
        help="Data directory.",
    ),
    workers: int = typer.Option(4, "--workers", "-w", help="Number of concurrent workers."),
):
    """Run or re-run a single dimension analysis."""
    db = _get_db(data_dir)
    cfg = AnalyzeConfig()

    # Validate dimension name
    if name not in cfg.all_dimensions:
        print_error(
            f"Unknown dimension: '{name}'",
            ValueError(f"Available: {list(cfg.all_dimensions.keys())}"),
        )
        raise typer.Exit(1)

    provider = LMStudioProvider()

    try:
        analyzer = DimensionAnalyzerService(provider, cfg)
        unanalyzed = db.get_unanalyzed_answers(name)

        if not unanalyzed:
            console.print(f"[yellow]All answers already analyzed for [{name}]. Nothing to do.[/yellow]")
            return

        print_header(f"Analyzing dimension: {name}")
        console.print(f"[dim]{len(unanalyzed)} answers to analyze | Workers: {workers}[/dim]")

        with _make_progress() as progress:
            task = progress.add_task(f"Analyzing [{name}]...", total=len(unanalyzed))
            count = analyzer.analyze_dimension(name, db, workers, progress, task)

        print_success(f"Analyzed {count} answers for [{name}]")
    finally:
        db.close()


@app.command("aggregate")
def run_aggregate(
    data_dir: Path = typer.Option(
        ".",
        "--data-dir", "-d",
        exists=True,
        file_okay=False,
        dir_okay=True,
        help="Data directory.",
    ),
):
    """Run step 6 aggregation across all analyzed answers."""
    db = _get_db(data_dir)
    cfg = AnalyzeConfig()
    provider = LMStudioProvider()

    try:
        aggregator = AggregationService(provider, cfg)
        enabled_dims = cfg.enabled_dimensions

        print_header("Cross-PDF Aggregation")
        console.print(f"[dim]Dimensions: {', '.join(enabled_dims)}[/dim]")

        with _make_progress() as progress:
            task = progress.add_task("Aggregating...", total=len(enabled_dims))
            count = aggregator.aggregate_all(db, progress, task)

        print_success(f"Aggregated {count} dimensions")
    finally:
        db.close()


@app.command("report")
def generate_report(
    data_dir: Path = typer.Option(
        ".",
        "--data-dir", "-d",
        exists=True,
        file_okay=False,
        dir_okay=True,
        help="Data directory.",
    ),
):
    """Generate final markdown + JSON report."""
    db = _get_db(data_dir)

    try:
        reporter = ReportGeneratorService()
        md_path, json_path = reporter.generate(db, data_dir)
        print_success(f"Markdown report: {md_path}")
        print_success(f"JSON report:     {json_path}")
    finally:
        db.close()


@app.command("reset")
def reset_pipeline(
    step: int = typer.Option(
        ...,
        "--step", "-s",
        help="Reset from this step onwards (1=images, 2=classification, 3=transcription, 4=segmentation, 5=dimensions, 6=aggregation).",
    ),
    data_dir: Path = typer.Option(
        ".",
        "--data-dir", "-d",
        exists=True,
        file_okay=False,
        dir_okay=True,
        help="Data directory.",
    ),
    yes: bool = typer.Option(False, "--yes", "-y", help="Skip confirmation."),
):
    """Reset pipeline from a given step onwards."""
    step_names = {
        1: "PDF → Images",
        2: "OCR Transcription",
        3: "Page Classification",
        4: "Answer Segmentation",
        5: "Dimension Analysis",
        6: "Aggregation",
    }

    if step not in step_names:
        print_error("Invalid step", ValueError(f"Step must be 1-6. Got: {step}"))
        raise typer.Exit(1)

    affected = [f"  {k}: {v}" for k, v in step_names.items() if k >= step]
    console.print(f"[bold yellow]This will reset the following steps:[/bold yellow]")
    for a in affected:
        console.print(f"[yellow]{a}[/yellow]")

    if not yes:
        confirm = typer.confirm("Proceed?")
        if not confirm:
            raise typer.Abort()

    db = _get_db(data_dir)
    try:
        db.reset_from_step(step)
        print_success(f"Reset from step {step} ({step_names[step]}) onwards")
    finally:
        db.close()

