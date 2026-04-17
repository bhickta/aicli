"""Video course command — GOD-MODE pipeline for full course archival."""
import typer
import re
import shutil
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn, TimeElapsedColumn

from aicli.cli.tui import print_header, console


def register(app: typer.Typer):
    """Register the course command on the given Typer app."""

    @app.command("course")
    def process_course(
        target_dir: Path = typer.Argument(
            ..., exists=True, file_okay=False, dir_okay=True,
            help="Path to the directory containing raw course videos."
        ),
        whisper_model: str = typer.Option(
            "large-v3", "--whisper-model", "-m",
            help="Whisper model for extremely accurate closed-captions."
        ),
        cleanup: str = typer.Option(
            "keep", "--cleanup",
            help="'keep' leaves intermediate files. 'trash' moves individual txt/srt/slideshows to a Trash folder."
        ),
        workers: int = typer.Option(
            4, "--workers", "-w",
            help="Parallel workers to use uniformly across all phases (Whisper, LM Studio, NVENC)."
        ),
        llm_model: str = typer.Option(
            None, "--llm-model", "--llm",
            help="Search string for the local model to dynamically load (e.g. 'gemma', 'llama3')."
        ),
        notes_llm: str = typer.Option(
            None, "--notes-llm",
            help="Search string for a BIGGER model for final notes generation (e.g. 'gemma-4-26b'). Uses --llm if not set."
        ),
    ):
        """
        GOD-MODE Pipeline: Turns a folder of raw videos into a single merged course pack.

        \\b
        Sequence:
        1. Transcribe (Full CC) + Text extraction (Whisper, max 2 GPU workers)
        2. AI Tagging & Intelligent Renaming (LM Studio, -w parallel workers)
        3. Compression into tiny 1-frame-per-minute slideshows (-w parallel workers)
        4. Merging: Stitches all videos, SRTs, and TXTs into single files.
        5. Clean Notes: Uses LM Studio to generate a single immense "No Fluff" MD file.
        """
        from aicli.cli.commands.video_processor import VideoBatchProcessor
        from aicli.services.video.compress_service import CompressService
        from aicli.services.video.merge_service import MergeService
        from aicli.services.video.notes_service import NotesService
        from aicli.services.video.tagger_service import VideoTaggerService
        from aicli.services.video.transcriber import WhisperEngine
        from aicli.config import resolve_dynamic_model, config as aicli_config

        console.print("[bold magenta]===== GOD-MODE COURSE PIPELINE INITIATED =====[/bold magenta]")

        # ── Locate valid raw files ──────────────────────────────────────
        valid_exts = VideoTaggerService.VIDEO_EXTENSIONS
        raw_files = [
            p for p in target_dir.rglob("*")
            if p.is_file() and p.suffix.lower() in valid_exts
            and "slideshow" not in p.name.lower()
            and "merged" not in p.name.lower()
        ]
        if not raw_files:
            console.print("[red]No raw videos found![/red]")
            return

        # ── Pre-flight: Scan all files and compute work stats ──────────
        from aicli.services.video.ffprobe import FFprobeClient
        from aicli.services.video.metadata_manager import MetadataBackupManager
        from rich.table import Table
        
        cache_dir = target_dir / ".aicli_cache"
        needs_transcription = []
        already_done = []
        needs_srt = 0
        needs_txt = 0
        needs_tagging = 0
        needs_rename = 0
        needs_compress = 0
        
        for f in raw_files:
            cache = MetadataBackupManager.load_cache(f)
            has_cache = "clips" in cache
            has_subs = FFprobeClient.has_subtitle_stream(f)
            
            if has_cache or has_subs:
                already_done.append(f)
            else:
                needs_transcription.append(f)
                needs_srt += 1
                needs_txt += 1
            
            if "ai_metadata" not in cache:
                needs_tagging += 1
            if "original_filename" not in cache:
                needs_rename += 1
            
            slideshow_name = f.stem + "_slideshow.mp4"
            if not (f.parent / slideshow_name).exists():
                needs_compress += 1

        # ── Display stats dashboard ──────────────────────────────────────
        stats = Table(title="Pre-Flight Pipeline Stats", show_header=True, header_style="bold cyan", border_style="dim")
        stats.add_column("Phase", style="bold")
        stats.add_column("Task", style="dim")
        stats.add_column("Pending", justify="right", style="yellow")
        stats.add_column("Done", justify="right", style="green")
        
        stats.add_row("1. Transcribe", "Whisper → SRT + TXT", str(len(needs_transcription)), str(len(already_done)))
        stats.add_row("2. Tag & Rename", "LM Studio → metadata + rename", str(needs_tagging), str(len(raw_files) - needs_tagging))
        stats.add_row("3. Compress", "NVENC → slideshow", str(needs_compress), str(len(raw_files) - needs_compress))
        stats.add_row("4. Merge", "FFmpeg concat", "—", "—")
        stats.add_row("5. Notes", "LM Studio → .md", "1" if not (target_dir / "Course_Merged_NoFluff.md").exists() else "0", "0" if not (target_dir / "Course_Merged_NoFluff.md").exists() else "1")
        console.print(stats)
        console.print()
        
        print_header(f"Phase 1: Transcribe {len(raw_files)} files")

        if needs_transcription:
            # ── Load Whisper only if there's actual work to do ────────────
            try:
                console.print(f"[cyan]Loading Whisper model on GPU ({whisper_model})...[/cyan]")
                whisper_workers = workers
                model_instance = WhisperEngine.load_whisper(whisper_model, num_workers=whisper_workers)
            except Exception as e:
                console.print(f"[red]Failed to load Whisper model: {e}[/red]")
                return

            # ════════════════════════════════════════════════════════════
            # PHASE 1: Transcribe remaining files
            # ════════════════════════════════════════════════════════════
            with Progress(
                SpinnerColumn(), TextColumn("[progress.description]{task.description}"),
                BarColumn(), TaskProgressColumn(), TimeElapsedColumn(), console=console
            ) as progress:
                task_p1 = progress.add_task(f"Transcribing ({whisper_workers} GPU workers)...", total=len(needs_transcription))
                with ThreadPoolExecutor(max_workers=whisper_workers) as executor:
                    futures = {
                        executor.submit(
                            VideoBatchProcessor.phase1_transcribe,
                            f, model_instance, write=True, full_cc=True,
                            retranscribe=False, transcribe_only=False,
                            clip_every=0, clip_len=0, save_txt=True,
                            progress=progress, task_id=task_p1
                        ): f for f in needs_transcription
                    }
                    for future in as_completed(futures):
                        path, err = future.result()
                        if err:
                            progress.console.print(f"[red]Error transcribing {futures[future].name}: {err}[/red]")
                        progress.advance(task_p1)

            # ── VRAM Purge ──────────────────────────────────────────────
            console.print("[cyan]Purging Whisper from VRAM...[/cyan]")
            try:
                del model_instance
                import torch, gc
                gc.collect()
                if torch.cuda.is_available():
                    torch.cuda.empty_cache()
            except Exception:
                pass
        else:
            console.print("[green]All files cached. Skipping Whisper entirely.[/green]")

        # ── Boot LM Studio into now-empty VRAM ──────────────────────────
        try:
            console.print("[cyan]Booting Language Model into VRAM...[/cyan]")
            resolved_lm = resolve_dynamic_model(llm_model)
            aicli_config.model_name = resolved_lm
            console.print(f"[green]✔ LM Studio ready: {resolved_lm}[/green]")
        except Exception as e:
            console.print(f"[dim]Note: Could not load LM Studio model ({e}). Continuing with JIT.[/dim]")

        # ════════════════════════════════════════════════════════════════
        # PHASE 2: AI Tagging & Renaming (-w parallel LM Studio workers)
        # ════════════════════════════════════════════════════════════════
        renamed_files = []
        tag_workers = workers
        print_header(f"Phase 2: Intelligent Tagging & Renaming ({tag_workers} workers)")
        with Progress(
            SpinnerColumn(), TextColumn("[progress.description]{task.description}"),
            BarColumn(), TaskProgressColumn(), TimeElapsedColumn(), console=console
        ) as progress:
            task_p2 = progress.add_task("Tagging and Renaming...", total=len(raw_files))
            with ThreadPoolExecutor(max_workers=tag_workers) as executor:
                futures = {
                    executor.submit(
                        VideoBatchProcessor.phase2_tag_and_mux,
                        f, write=True, no_rename=False, text_thumb=False,
                        retranscribe=False, transcribe_only=False,
                        progress=progress, task_id=task_p2
                    ): f for f in raw_files
                }
                for future in as_completed(futures):
                    new_path, _, err = future.result()
                    if err:
                        progress.console.print(f"[red]Error tagging {futures[future].name}: {err}[/red]")
                    else:
                        renamed_files.append(new_path)
                    progress.advance(task_p2)

        # ── Sort by first number in filename for correct order ──────────
        def extract_number(path: Path) -> int:
            match = re.search(r'\d+', path.name)
            return int(match.group()) if match else 999
        renamed_files.sort(key=extract_number)

        # ════════════════════════════════════════════════════════════════
        # PHASE 3: Slideshow Compression (-w parallel FFmpeg workers)
        # ════════════════════════════════════════════════════════════════
        print_header(f"Phase 3: Slideshow Compression ({workers} workers)")
        slideshow_files = []
        with Progress(
            SpinnerColumn(), TextColumn("[progress.description]{task.description}"),
            BarColumn(), TaskProgressColumn(), TimeElapsedColumn(), console=console
        ) as progress:
            task_p3 = progress.add_task("GPU NVENC Fast-Skipping...", total=len(renamed_files))
            with ThreadPoolExecutor(max_workers=workers) as executor:
                futures = {
                    executor.submit(
                        CompressService.compress,
                        f, None, 0, "slideshow", False, None, "1/60", True
                    ): f for f in renamed_files
                }
                for future in as_completed(futures):
                    try:
                        out_path = future.result()
                        slideshow_files.append(out_path)
                        progress.console.print(f"[green]Compressed → {out_path.name}[/green]")
                    except Exception as e:
                        progress.console.print(f"[red]Compression failed: {e}[/red]")
                    progress.advance(task_p3)
        slideshow_files.sort(key=extract_number)

        # ════════════════════════════════════════════════════════════════
        # PHASE 4: Deep Native Merging
        # ════════════════════════════════════════════════════════════════
        print_header("Phase 4: Deep Native Merging")
        cache_dir = target_dir / ".aicli_cache"
        merged_vid = target_dir / "Course_Merged_Slideshow.mp4"
        merged_srt = target_dir / "Course_Merged.srt"
        merged_txt = target_dir / "Course_Merged.txt"
        merged_md  = target_dir / "Course_Merged_NoFluff.md"

        console.print("[cyan]Stitching videos losslessly...[/cyan]")
        if MergeService.merge_videos(slideshow_files, merged_vid):
            console.print(f"[bold green]✔ Saved {merged_vid.name}[/bold green]")

        console.print("[cyan]Time-shifting and merging SRTs...[/cyan]")
        video_srt_pairs = []
        for v in slideshow_files:
            base = v.name.replace("_slideshow", "").replace(v.suffix, "")
            srt = cache_dir / f"{base}.srt"
            if not srt.exists():
                srt = cache_dir / f"{base}.tmp_cc.srt"
            video_srt_pairs.append((v, srt))
        if MergeService.merge_srts(video_srt_pairs, merged_srt):
            console.print(f"[bold green]✔ Saved {merged_srt.name}[/bold green]")

        console.print("[cyan]Appending raw text transcripts...[/cyan]")
        txt_files = [cache_dir / f"{v.name.replace('_slideshow', '').replace(v.suffix, '')}.txt" for v in slideshow_files]
        if MergeService.merge_txts(txt_files, merged_txt):
            console.print(f"[bold green]✔ Saved {merged_txt.name}[/bold green]")

        # ════════════════════════════════════════════════════════════════
        # PHASE 5: LM Studio 'No Fluff' Notes (hot-swap to bigger model)
        # ════════════════════════════════════════════════════════════════
        print_header("Phase 5: LM Studio 'No Fluff' Clean Transcription")
        if merged_txt.exists():
            if notes_llm:
                try:
                    console.print(f"[cyan]Hot-swapping to heavier model: '{notes_llm}'...[/cyan]")
                    resolved_notes = resolve_dynamic_model(notes_llm)
                    aicli_config.model_name = resolved_notes
                    console.print(f"[green]✔ Loaded notes model: {resolved_notes}[/green]")
                except Exception as e:
                    console.print(f"[dim]Could not load notes model ({e}). Using current model.[/dim]")
            console.print("[cyan]Streaming merged transcript through LM Studio...[/cyan]")
            try:
                full_text = merged_txt.read_text(encoding="utf-8")
                notes_content = NotesService.generate_notes_from_text(full_text, style="clean")
                merged_md.write_text(notes_content, encoding="utf-8")
                console.print(f"[bold green]✔ Notes saved → {merged_md.name}[/bold green]")
            except Exception as e:
                console.print(f"[red]Failed to generate notes: {e}[/red]")

        # ════════════════════════════════════════════════════════════════
        # PHASE 6: Cleanup
        # ════════════════════════════════════════════════════════════════
        if cleanup == "trash":
            print_header("Phase 6: Cleaning up intermediate files")
            trash_dir = target_dir / "Trash"
            trash_dir.mkdir(exist_ok=True)
            count = 0
            for f in slideshow_files:
                base = f.name.replace("_slideshow", "").replace(f.suffix, "")
                srt_f = cache_dir / f"{base}.srt"
                txt_f = cache_dir / f"{base}.txt"
                if f.exists(): shutil.move(str(f), str(trash_dir / f.name)); count += 1
                if srt_f.exists(): shutil.move(str(srt_f), str(trash_dir / srt_f.name)); count += 1
                if txt_f.exists(): shutil.move(str(txt_f), str(trash_dir / txt_f.name)); count += 1
            console.print(f"[dim]Moved {count} intermediate files to Trash folder.[/dim]")

        console.print("\n[bold magenta]===== GOD-MODE PIPELINE COMPLETE =====[/bold magenta]")
