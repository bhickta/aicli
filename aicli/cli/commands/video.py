import typer
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
import time

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn, TimeElapsedColumn, TimeRemainingColumn
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
    transcribe_only: bool = typer.Option(
        False,
        "--transcribe-only",
        help="Skip AI tagging and file renaming. Only perform transcription (writes .srt if --full-cc is used)."
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
    save_txt: bool = typer.Option(
        False,
        "--save-txt",
        help="Save a clean plain-text transcript as a .txt file alongside the video."
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
            console.print(f"[cyan]Loading Whisper model on GPU (batch_size=24)...[/cyan]")
            model_instance = WhisperEngine.load_whisper(whisper_model, num_workers=workers)
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
                f, 
                model_instance, 
                write, 
                no_rename,
                full_cc,
                text_thumb,
                retranscribe, 
                transcribe_only,
                clip_every, 
                clip_len, 
                save_txt,
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
                    if not write and not transcribe_only:
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
        BarColumn(),
        TaskProgressColumn(),
        TimeElapsedColumn(),
        TextColumn("•"),
        TimeRemainingColumn(),
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


@app.command("notes")
def generate_notes(
    target_path: Path = typer.Argument(
        ...,
        exists=True,
        help="Path to a video file or directory. Processes videos with sidecar .srt files OR embedded subtitle streams."
    ),
    overwrite: bool = typer.Option(
        False,
        "--overwrite",
        help="Overwrite existing .md notes files."
    ),
    style: str = typer.Option(
        "bullet",
        "--style", "-s",
        help="Note style: 'bullet' (ultra-dense, default) or 'clean' (removes fluff, no info loss)."
    ),
):
    """
    Generate ultra-dense exam-ready study notes from SRT transcripts via LM Studio.

    Detects subtitles from sidecar .srt files or embedded subtitle streams inside the
    video container. Converts to plain text, sends to LM Studio, saves as .md file.
    """
    from aicli.services.video.notes_service import NotesService
    
    valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS + (".txt", ".srt")
    if target_path.is_file():
        files = [target_path] if target_path.suffix.lower() in valid_extensions else []
    else:
        files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

    if not files:
        console.print("[yellow]No supported files found (.mp4, .mkv, .txt, .srt).[/yellow]")
        raise typer.Exit()

    # Filter: txt, sidecar .srt exists OR embedded subtitle stream detected
    console.print(f"[dim]Scanning {len(files)} file(s) for transcripts...[/dim]")
    eligible = []  # list of (target_file, source_type)
    for f in files:
        md = f.with_suffix(".md")
        if md.exists() and not overwrite:
            console.print(f"[dim]\\[{f.name}] Notes already exist, skipping (use --overwrite)[/dim]")
            continue

        if f.suffix.lower() == ".txt":
            eligible.append((f, "txt"))
        elif f.suffix.lower() == ".srt":
            eligible.append((f, "txt_srt"))
        else:
            srt = f.with_suffix(".srt")
            if srt.exists():
                eligible.append((f, "sidecar"))
            elif NotesService.has_subtitle_stream(f):
                eligible.append((f, "embedded"))

    if not eligible:
        console.print("[yellow]No transcripts found (or all already have notes).[/yellow]")
        raise typer.Exit()

    print_header(f"Generating notes for {len(eligible)} video(s)")
    console.print("[dim]LM Studio must be running with a model loaded.[/dim]\n")

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
        task_id = progress.add_task("Generating notes...", total=len(eligible))
        
        successes = 0
        for f, source_type in eligible:
            tmp_extracted = None
            try:
                if source_type == "txt":
                    progress.console.print(f"[cyan]\\[{f.name}] Reading native .txt → passing to LM Studio...[/cyan]")
                    text = f.read_text(encoding="utf-8", errors="replace")
                    notes = NotesService.generate_notes_from_text(text, style=style)
                elif source_type == "txt_srt":
                    progress.console.print(f"[cyan]\\[{f.name}] Reading native .srt → passing to LM Studio...[/cyan]")
                    notes = NotesService.generate_notes(f, style=style)
                else:
                    if source_type == "sidecar":
                        srt_path = f.with_suffix(".srt")
                        progress.console.print(f"[cyan]\\[{f.name}] Reading sidecar .srt → passing to LM Studio...[/cyan]")
                    else:
                        progress.console.print(f"[cyan]\\[{f.name}] Extracting embedded CC → passing to LM Studio...[/cyan]")
                        srt_path = NotesService.extract_srt_from_video(f)
                        if not srt_path:
                            progress.console.print(f"[red]\\[{f.name}] Failed to extract subtitle stream.[/red]")
                            progress.advance(task_id)
                            continue
                        tmp_extracted = srt_path  # Mark for cleanup
                    notes = NotesService.generate_notes(srt_path, style=style)
                
                md_path = NotesService.save_notes(f, notes)
                progress.console.print(f"[bold green]\\[{f.name}] Notes saved → {md_path.name}[/bold green]")
                successes += 1
            except Exception as e:
                progress.console.print(f"[red]\\[{f.name}] Error: {e}[/red]")
            finally:
                # Clean up temp extracted SRT
                if tmp_extracted and tmp_extracted.exists():
                    tmp_extracted.unlink()
            progress.advance(task_id)

    if successes:
        print_success(f"Generated notes for {successes}/{len(eligible)} videos.")
    else:
        print_error("No notes were generated.", RuntimeError("All files failed."))


@app.command("compress")
def compress_video(
    target_path: Path = typer.Argument(
        ...,
        exists=True,
        help="Path to a video file or directory."
    ),
    resolution: int = typer.Option(
        240,
        "--res", "-r",
        help="Target vertical resolution (e.g. 240, 360, 480)."
    ),
    preset: str = typer.Option(
        "light",
        "--preset", "-p",
        help="Compression preset: ultralight, light, balanced, slideshow."
    ),
    overwrite: bool = typer.Option(
        False,
        "--overwrite",
        help="Replace the original file with the compressed version."
    ),
    workers: int = typer.Option(
        4,
        "--workers",
        help="Number of concurrent compression jobs."
    ),
    crf: int = typer.Option(
        None,
        "--crf",
        help="Constant quality (0-51). Lower = better. Overrides preset bitrate."
    ),
    fps: str = typer.Option(
        None,
        "--fps",
        help="Override output framerate (e.g., 5, 1, or '1/60' for 1 frame per minute)."
    ),
    fast_skip: bool = typer.Option(
        False,
        "--fast-skip",
        help="Skip decoding non-keyframes. Drops 5 hours down to ~30 seconds for slideshows, at the expense of exact frame accuracy."
    ),
):
    """
    GPU-accelerated video compression using NVENC (RTX 3090).

    Full GPU-resident pipeline: decode → scale → encode all on GPU.
    Frames never leave VRAM. A 2-hour lecture compresses in ~15 seconds.

    \b
    Examples:
        aicli video compress ./                       # 240p, 15fps, light
        aicli video compress ./ --preset ultralight    # Smallest: 10fps, 150kbps
        aicli video compress ./ --fps 5                # Absolute minimum FPS
        aicli video compress ./ --overwrite            # Replace originals
    """
    from aicli.services.video.compress_service import CompressService

    valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS
    if target_path.is_file():
        files = [target_path] if target_path.suffix.lower() in valid_extensions else []
    else:
        files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

    if not files:
        console.print("[yellow]No supported video files found.[/yellow]")
        raise typer.Exit()

    # Skip already-compressed files (those ending with _240p.mp4 etc.)
    if not overwrite:
        files = [f for f in files if f"_{resolution}p" not in f.stem]

    if not files:
        console.print("[yellow]All files already compressed. Use --overwrite to reprocess.[/yellow]")
        raise typer.Exit()

    print_header(f"Compressing {len(files)} video(s) → {resolution}p [{preset}]")
    from aicli.services.video.compress_service import CompressService
    display_fps = fps if fps is not None else CompressService.PRESETS[preset][4]
    console.print(f"[dim]Workers: {workers} | Encoder: h264_nvenc (GPU) | Preset: {preset} | FPS: {display_fps} | Pipeline: full VRAM[/dim]")
    if overwrite:
        console.print("[bold red]WARNING: --overwrite is ON. Original files will be REPLACED.[/bold red]\n")
    else:
        console.print("[dim]Compressed files saved alongside originals as *_240p.mp4[/dim]\n")

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
        task_id = progress.add_task("Compressing...", total=len(files))

        executor = ThreadPoolExecutor(max_workers=workers)
        futures = {
            executor.submit(
                _compress_single, f, resolution, preset, overwrite, crf, fps, fast_skip
            ): f for f in files
        }

        for future in as_completed(futures):
            src = futures[future]
            try:
                out_path, src_mb, out_mb = future.result()
                ratio = (1 - out_mb / src_mb) * 100 if src_mb > 0 else 0
                progress.console.print(
                    f"[bold green]✔ {src.name}[/bold green] "
                    f"[dim]{src_mb:.1f}MB → {out_mb:.1f}MB ({ratio:.0f}% smaller)[/dim]"
                )
                results.append((src, out_path, None))
            except Exception as e:
                progress.console.print(f"[red]✖ {src.name}: {e}[/red]")
                results.append((src, None, e))
            progress.advance(task_id)

    elapsed = time.time() - start_t
    successes = [r for r in results if not r[2]]
    failures = [r for r in results if r[2]]

    console.print(f"\n[bold]Done in {elapsed:.1f}s[/bold]")
    if successes:
        total_src = sum(CompressService.get_file_size_mb(r[0]) for r in successes if r[0].exists())
        total_dst = sum(CompressService.get_file_size_mb(r[1]) for r in successes if r[1] and r[1].exists())
        print_success(
            f"Compressed {len(successes)}/{len(files)} files. "
            f"Total: {total_src:.1f}MB → {total_dst:.1f}MB"
        )
    if failures:
        print_error(f"Failed: {len(failures)} files.", failures[0][2])


def _compress_single(
    video_path: Path, resolution: int, preset: str, overwrite: bool, crf: int, fps: str, fast_skip: bool
) -> tuple:
    """Worker function for parallel compression."""
    from aicli.services.video.compress_service import CompressService

    src_mb = CompressService.get_file_size_mb(video_path)
    out_path = CompressService.compress(
        video_path,
        resolution=resolution,
        preset=preset,
        overwrite=overwrite,
        crf=crf,
        fps=fps,
        fast_skip=fast_skip,
    )
    out_mb = CompressService.get_file_size_mb(out_path)
    return out_path, src_mb, out_mb

