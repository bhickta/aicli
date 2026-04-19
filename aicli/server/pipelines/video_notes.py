"""Video notes command — Generate study notes from SRT transcripts."""
import typer
from pathlib import Path

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn, TimeElapsedColumn, TimeRemainingColumn

from aicli.services.video import VideoTaggerService
from aicli.cli.tui import print_header, print_success, print_error, console


def process_notes(target_path: Path, overwrite: bool = False, style: str = "bullet"):
    """Core logic to generate video notes, extracted for API usage."""

def register(app: typer.Typer):
    """Register the notes command on the given Typer app."""

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
        process_notes(target_path, overwrite, style)

def process_notes(target_path: Path, overwrite: bool = False, style: str = "bullet"):
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
