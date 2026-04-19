"""Video compress command — GPU-accelerated video compression."""
import typer
import time
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn, TimeElapsedColumn, TimeRemainingColumn

from aicli.services.video import VideoTaggerService
from aicli.cli.tui import print_header, print_success, print_error, console


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


def register(app: typer.Typer):
    """Register the compress command on the given Typer app."""

    @app.command("compress")
    def compress_video(
        target_path: Path = typer.Argument(
            ...,
            exists=True,
            help="Path to a video file or directory."
        ),
        resolution: int = typer.Option(240, "--res", "-r", help="Target vertical resolution (e.g. 240, 360, 480)."),
        preset: str = typer.Option("light", "--preset", "-p", help="Compression preset: ultralight, light, balanced, slideshow."),
        overwrite: bool = typer.Option(False, "--overwrite", help="Replace the original file with the compressed version."),
        workers: int = typer.Option(4, "--workers", help="Number of concurrent compression jobs."),
        crf: int = typer.Option(None, "--crf", help="Constant quality (0-51). Lower = better. Overrides preset bitrate."),
        fps: str = typer.Option(None, "--fps", help="Override output framerate (e.g., 5, 1, or '1/60' for 1 frame per minute)."),
        fast_skip: bool = typer.Option(False, "--fast-skip", help="Skip decoding non-keyframes for ultra-fast slideshows."),
    ):
        """
        GPU-accelerated video compression using NVENC (RTX 3090).

        Full GPU-resident pipeline: decode → scale → encode all on GPU.
        Frames never leave VRAM. A 2-hour lecture compresses in ~15 seconds.

        \\b
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

        if not overwrite:
            files = [f for f in files if f"_{resolution}p" not in f.stem]

        if not files:
            console.print("[yellow]All files already compressed. Use --overwrite to reprocess.[/yellow]")
            raise typer.Exit()

        print_header(f"Compressing {len(files)} video(s) → {resolution}p [{preset}]")
        display_fps = fps if fps is not None else CompressService.PRESETS[preset][4]
        console.print(f"[dim]Workers: {workers} | Encoder: h264_nvenc (GPU) | Preset: {preset} | FPS: {display_fps} | Pipeline: full VRAM[/dim]")
        if overwrite:
            console.print("[bold red]WARNING: --overwrite is ON. Original files will be REPLACED.[/bold red]\n")
        else:
            console.print("[dim]Compressed files saved alongside originals as *_240p.mp4[/dim]\n")

        results = []
        start_t = time.time()

        with Progress(
            SpinnerColumn(), TextColumn("[progress.description]{task.description}"),
            BarColumn(), TaskProgressColumn(), TimeElapsedColumn(),
            TextColumn("•"), TimeRemainingColumn(), console=console
        ) as progress:
            task_id = progress.add_task("Compressing...", total=len(files))
            executor = ThreadPoolExecutor(max_workers=workers)
            futures = {
                executor.submit(_compress_single, f, resolution, preset, overwrite, crf, fps, fast_skip): f
                for f in files
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
