"""Video tag command — Transcribe and AI-tag lecture videos."""
import typer
import time
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn, TimeElapsedColumn, TimeRemainingColumn

from aicli.services.video import MetadataBackupManager, WhisperEngine, VideoTaggerService
from aicli.cli.commands.video_processor import VideoBatchProcessor
from aicli.cli.tui import print_header, print_success, print_error, console

def tag_video(
    target_path: Path,
    write: bool = False,
    no_rename: bool = False,
    full_cc: bool = False,
    text_thumb: bool = True,
    retranscribe: bool = False,
    transcribe_only: bool = False,
    workers: int = 2,
    clip_every: int = 360,
    clip_len: int = 60,
    save_txt: bool = False,
    whisper_model: str = "base"
):
    """
    Transcribe and AI-tag lecture videos. Parallel, cached, and automatically backed up.
        """
        valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS
        if target_path.is_file():
            files = [target_path] if target_path.suffix.lower() in valid_extensions else []
        else:
            files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

        if not files:
            console.print("[yellow]No supported video files found.[/yellow]")
            return

        print_header(f"Found {len(files)} video(s) to process")
        console.print(f"[dim]Workers: {workers} | Whisper: {whisper_model} | Cache: {'IGNORED' if retranscribe else 'Enabled'}[/dim]")

        if write:
            console.print("[bold red]WARNING: Running in WRITE mode. Files will be tagged and renamed.[/bold red]\n")
        else:
            console.print("[bold blue]Running in DRY RUN mode. Provide --write to actually modify files.[/bold blue]\n")

        # Only load Whisper if at least one file needs transcribing
        model_instance = None
        all_cached = all("clips" in MetadataBackupManager.load_cache(f) for f in files) and not retranscribe
        
        if not all_cached:
            try:
                console.print(f"[cyan]Loading Whisper model on GPU (batch_size=24)...[/cyan]")
                model_instance = WhisperEngine.load_whisper(whisper_model, num_workers=workers)
            except Exception as e:
                print_error("Failed to load whisper model", e)
                return
        else:
            console.print("[green]All files cached. Skipping Whisper instantiation.[/green]")

        results = []
        start_t = time.time()

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TaskProgressColumn(),
            TimeElapsedColumn(),
            TextColumn("•"),
            TimeRemainingColumn(),
            console=console
        ) as progress:
            task_id = progress.add_task("Processing videos...", total=len(files))

            executor = ThreadPoolExecutor(max_workers=workers)
            futures = [
                executor.submit(
                    VideoBatchProcessor.process_isolated_video, 
                    f, model_instance, write, no_rename,
                    full_cc, text_thumb, retranscribe, transcribe_only,
                    clip_every, clip_len, save_txt, progress, task_id
                ) 
                for f in files
            ]
            
            try:
                for future in as_completed(futures):
                    f_path, metadata, err = future.result()
                    
                    if err:
                        progress.console.print(f"[{f_path.name}] [red]Error: {str(err)}[/red]")
                    else:
                        new_name = (metadata.get('filename') or '')
                        if not write and not transcribe_only:
                            progress.console.print(f"[{f_path.name}] [cyan][DRY RUN] Would tag + rename to: {new_name}{f_path.suffix}[/cyan]")
                    
                    results.append((f_path, metadata, err))
                    progress.advance(task_id)
            except KeyboardInterrupt:
                progress.console.print("\n[bold red]⚠ Processing interrupted by user! Shutting down abruptly...[/bold red]")
                import os
                os._exit(130)

        elapsed = time.time() - start_t
        successful = [r for r in results if not r[2]]
        failures = [r for r in results if r[2]]

        console.print(f"\n[bold]Done in {elapsed:.1f}s[/bold]")
        if successful:
            print_success(f"Processed {len(successful)}/{len(files)} files successfully.")
        if failures:
            print_error(f"Failed to process {len(failures)} files.", failures[0][2])
            for f, _, e in failures:
                console.print(f"  - {f.name}: {e}")

        if not write:
             console.print("\n[yellow]Run with --write to apply tags and rename.[/yellow]")
