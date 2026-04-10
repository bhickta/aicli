import typer
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
import time

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn
from rich.table import Table

from aicli.services.video import FFprobeClient, MetadataBackupManager, WhisperEngine, VideoTaggerService
from aicli.cli.commands.video_processor import VideoBatchProcessor
from aicli.cli.tui import print_header, print_success, print_error, console

app = typer.Typer(help="Video transcription and metadata tagging commands.")



@app.command("tag")
def tag_video(
    target_path: Path = typer.Argument(
        ...,
        exists=True,
        help="Path to the video file or directory."
    ),
    write: bool = typer.Option(
        False,
        "--write", "-w",
        help="Apply tags and rename the file. Without this, it performs a dry run."
    ),
    no_rename: bool = typer.Option(
        False,
        "--no-rename",
        help="Tag only, keep the original filename. (Requires --write)"
    ),
    full_cc: bool = typer.Option(
        False,
        "--full-cc",
        help="Perform a full transcription to generate a sidecar .srt track."
    ),
    text_thumb: bool = typer.Option(
        True,
        "--text-thumb/--no-text-thumb",
        help="Generate a centered text image from the title and embed it directly into the video as cover art."
    ),
    retranscribe: bool = typer.Option(
        False,
        "--retranscribe",
        help="Ignore cached sidecar data and force a full re-transcription."
    ),
    workers: int = typer.Option(
        2,
        "--workers",
        help="Number of concurrent workers."
    ),
    clip_every: int = typer.Option(
        360,
        "--clip-every",
        help="Frequency to sample audio clips (in seconds)."
    ),
    clip_len: int = typer.Option(
        60,
        "--clip-len",
        help="Duration of each audio sample (in seconds)."
    ),
    whisper_model: str = typer.Option(
        "base",
        "--whisper-model",
        help="faster-whisper model size (tiny, base, small, medium, large-v3)."
    )
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
        raise typer.Exit()

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
            console.print("[cyan]Loading Whisper model on GPU...[/cyan]")
            model_instance = WhisperEngine.load_whisper(whisper_model)
        except Exception as e:
            print_error("Failed to load whisper model", e)
            raise typer.Exit(1)
    else:
        console.print("[green]All files cached. Skipping Whisper instantiation.[/green]")

    results = []
    
    start_t = time.time()

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TaskProgressColumn(),
        console=console
    ) as progress:
        task_id = progress.add_task("Processing videos...", total=len(files))

        executor = ThreadPoolExecutor(max_workers=workers)
        futures = [
            executor.submit(
                VideoBatchProcessor.process_isolated_video, 
                f, 
                model_instance, 
                write, 
                no_rename,
                full_cc,
                text_thumb,
                retranscribe, 
                clip_every, 
                clip_len, 
                progress, 
                task_id
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
                    if not write:
                        progress.console.print(f"[{f_path.name}] [cyan][DRY RUN] Would tag + rename to: {new_name}{f_path.suffix}[/cyan]")
                
                results.append((f_path, metadata, err))
                progress.advance(task_id)
        except KeyboardInterrupt:
            progress.console.print("\n[bold red]⚠ Processing interrupted by user! Shutting down abruptly...[/bold red]")
            import os
            os._exit(130)  # Instantly kills process, avoiding C++ CUDA destructor core dumps

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
         


@app.command("restore")
def restore_tags(
    target_path: Path = typer.Argument(
        ...,
        exists=True,
        help="Path to the video file or directory to restore."
    )
):
    """
    Restore original tags from the backup sidecar (undoing any --write actions).
    """
    valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS
    if target_path.is_file():
        files = [target_path] if target_path.suffix.lower() in valid_extensions else []
    else:
        files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

    if not files:
        console.print("[yellow]No supported video files found.[/yellow]")
        raise typer.Exit()
        
    print_header(f"Restoring {len(files)} files")
    
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console
    ) as progress:
        task_id = progress.add_task("Restoring backup sidecars...", total=len(files))
        for f in files:
            try:
                if MetadataBackupManager.restore_original_tags(f):
                    progress.console.print(f"[green]✔ Restored {f.name}[/green]")
                else:
                    progress.console.print(f"[yellow]⚠ No backup tags to restore in {f.name}[/yellow]")
            except Exception as e:
                progress.console.print(f"[red]✖ Failed to restore {f.name}: {e}[/red]")
            progress.advance(task_id)

    print_success("Restore operation complete.")

@app.command("info")
def view_metadata(
    target_path: Path = typer.Argument(
        ...,
        exists=True,
        help="Path to the video file or directory."
    )
):
    """
    View current metadata of a video file.
    """
    valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS
    if target_path.is_file():
        files = [target_path] if target_path.suffix.lower() in valid_extensions else []
    else:
        files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

    if not files:
        console.print("[yellow]No supported video files found.[/yellow]")
        raise typer.Exit()
        
    for f in files:
        tags = FFprobeClient.read_existing_tags(f)
        if not tags:
            console.print(f"\n[cyan]{f.name}[/cyan]\n[yellow]No metadata found.[/yellow]")
            continue
            
        table = Table(title=f"Metadata: {f.name}", show_header=True, header_style="bold magenta")
        table.add_column("Property", style="dim", width=20)
        table.add_column("Value")
        
        for k, v in tags.items():
            if k.lower() == "aicli_backup":
                table.add_row(k, "[dim italic]<Embedded Backup Payload>[/dim italic]")
            elif isinstance(v, list):
                table.add_row(k, ", ".join(map(str, v)))
            else:
                table.add_row(k, str(v))
                
        console.print(table)
        console.print()

