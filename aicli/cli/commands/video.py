import typer
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
import shutil
import time

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn
from rich.table import Table

from aicli.services.video_tagger import VideoTaggerService
from aicli.cli.tui import print_header, print_success, print_error, confirm_action, console

app = typer.Typer(help="Video transcription and metadata tagging commands.")


def _process_video(video_path: Path, whisper_model, write: bool, no_rename: bool, generate_thumb: bool, retranscribe: bool, clip_every: int, clip_len: int, progress: Progress, task_id) -> tuple[Path, dict, Exception]:
    """Helper to process a single video in a worker thread."""
    try:
        cache = VideoTaggerService.load_cache(video_path) if not retranscribe else {}
        VideoTaggerService.backup_original_tags(video_path, cache)
        VideoTaggerService.save_cache(video_path, cache)

        if "clips" in cache and not retranscribe:
            clips = cache["clips"]
            progress.console.print(f"[dim]\[{video_path.name}] Loaded cached transcript[/dim]")
        else:
            progress.console.print(f"[cyan]\[{video_path.name}] Transcribing audio...[/cyan]")
            
            def log_callback(total, current, t, text):
                progress.console.print(f"[dim]  [{current}/{total}] @{int(t//60)}m{int(t%60)}s: {text[:60]}...[/dim]")
                
            clips = VideoTaggerService.transcribe_video(video_path, whisper_model, clip_every, clip_len, callback=log_callback)
            if not clips:
                return video_path, None, ValueError("No speech detected.")
                
            cache["clips"] = clips
            VideoTaggerService.save_cache(video_path, cache)

        if "ai" in cache and not retranscribe:
            ai = cache["ai"]
            progress.console.print(f"[dim]\[{video_path.name}] Loaded cached AI tags[/dim]")
        else:
            progress.console.print(f"[cyan]\[{video_path.name}] Requesting metadata from LM Studio...[/cyan]")
            ai = VideoTaggerService.ask_lmstudio(clips, str(video_path.parent))
            if not ai:
                return video_path, None, ValueError("LM Studio returned empty response.")
                
            cache["ai"] = ai
            VideoTaggerService.save_cache(video_path, cache)

        progress.console.print(f"[green]\[{video_path.name}] Evaluated: {ai.get('title')} ({ai.get('subject')})[/green]")

        new_tags = {
            "title":       ai.get("title", ""),
            "comment":     ai.get("description", ""),
            "genre":       ai.get("subject", ""),
            "description": ai.get("description", ""),
            "SUBJECT":     ai.get("subject", ""),
            "TOPICS":      ", ".join(ai.get("topics", [])),
            "LANGUAGE":    ai.get("language", ""),
            "SUMMARY":     ai.get("description", ""),
        }

        if write:
            # Pass original_tags to embed them into the video natively
            VideoTaggerService.write_tags(video_path, new_tags, original_tags=cache.get("original_tags"))
            progress.console.print(f"[{video_path.name}] [bold green]Tags and backup embedded natively.[/bold green]")

            # Clean up temporary sidecar cache since everything is embedded now
            sc_path = VideoTaggerService.sidecar_path(video_path)
            if sc_path.exists():
                sc_path.unlink()

            if not no_rename:
                new_name = (ai.get("filename") or "").strip()
                if new_name:
                    if not new_name.endswith(video_path.suffix):
                        new_name += video_path.suffix
                    new_path = video_path.parent / new_name
                    if new_path != video_path and not new_path.exists():
                        shutil.move(str(video_path), str(new_path))
                        progress.console.print(f"[{video_path.name}] [bold blue]Renamed → {new_name}[/bold blue]")
                        video_path = new_path # Update reference for thumbnail extraction

            if generate_thumb:
                thumb_path = video_path.with_suffix(".jpg")
                if VideoTaggerService.generate_thumbnail(video_path, thumb_path):
                    progress.console.print(f"[{video_path.name}] [bold magenta]Thumbnail generated: {thumb_path.name}[/bold magenta]")

        return video_path, ai, None
    except Exception as e:
        return video_path, None, e


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
    thumb: bool = typer.Option(
        True,
        "--thumb/--no-thumb",
        help="Automatically generate a .jpg thumbnail alongside the video."
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
    all_cached = all("clips" in VideoTaggerService.load_cache(f) for f in files) and not retranscribe
    
    if not all_cached:
        try:
            console.print("[cyan]Loading Whisper model on GPU...[/cyan]")
            model_instance = VideoTaggerService.load_whisper(whisper_model)
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
                _process_video, 
                f, 
                model_instance, 
                write, 
                no_rename,
                thumb,
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
            executor.shutdown(wait=False, cancel_futures=True)
            raise typer.Exit(code=130)
        finally:
            # Ensure proper cleanup if it wasn't intercepted
            executor.shutdown(wait=False)

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
                if VideoTaggerService.restore_original_tags(f):
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
        tags = VideoTaggerService.read_existing_tags(f)
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

