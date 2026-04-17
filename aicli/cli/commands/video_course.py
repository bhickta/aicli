"""Video course command — GOD-MODE pipeline for full course archival."""
import typer
import re
import shutil
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn, TimeElapsedColumn
from rich.status import Status

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
        w1: int = typer.Option(
            2, "--w1",
            help="Parallel workers for Phase 1 (Whisper Transcription). Maxed by GPU VRAM."
        ),
        w2: int = typer.Option(
            12, "--w2",
            help="Parallel workers for Phase 2 (LM Studio Tagging & Renaming)."
        ),
        w3: int = typer.Option(
            12, "--w3",
            help="Parallel workers for Phase 3 (NVENC Compression)."
        ),
        llm_model: str = typer.Option(
            None, "--llm-model", "--llm",
            help="Search string for the local model to dynamically load (e.g. 'gemma', 'llama3')."
        ),
        max_merge_hours: float = typer.Option(
            0.0, "--max-merge-hours",
            help="Maximum length (in hours) per merged video. If exceeded, splits into Part1, Part2, etc. (0 = no limit)."
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
            and ".aicli_cache" not in str(p)
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
        
        from aicli.services.video.notes_service import NotesService
        import subprocess
        for f in raw_files:
            cache = MetadataBackupManager.load_cache(f)
            has_cache = "clips" in cache
            has_subs = FFprobeClient.has_subtitle_stream(f)
            
            ext_srt = f.parent / ".aicli_cache" / f"{f.stem}.srt"
            ext_txt = f.parent / ".aicli_cache" / f"{f.stem}.txt"
            
            # Ensure physical text files are forcefully present in the cache for Phase 4 merging
            if has_subs and not ext_srt.exists():
                subprocess.run(["ffmpeg", "-y", "-v", "quiet", "-i", str(f), "-map", "0:s:0", "-c:s", "srt", str(ext_srt)])
                
            if ext_srt.exists() and not ext_txt.exists():
                try:
                    clean_text = NotesService.srt_to_text(ext_srt)
                    ext_txt.write_text(clean_text, encoding="utf-8")
                except Exception:
                    ext_txt.write_text("", encoding="utf-8")
            
            if (has_cache or has_subs) and ext_srt.exists() and ext_txt.exists():
                already_done.append(f)
            else:
                needs_transcription.append(f)
                needs_srt += 1
                needs_txt += 1
            
            if "ai" not in cache:
                needs_tagging += 1
            if "original_filename" not in cache:
                needs_rename += 1
            
            ai_tags = cache.get("ai", {})
            target_name = ai_tags.get("filename", f.stem)
            slideshow_name = f"{target_name}_slideshow.mp4"
            if not (f.parent / ".aicli_cache" / "slideshows" / slideshow_name).exists():
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
                whisper_workers = w1
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
        tag_workers = w2
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
                        renamed_files.append(futures[future]) # Fallback to original file
                    else:
                        renamed_files.append(new_path)
                    progress.advance(task_p2)

        # ── Global AI Sorting ──────────
        progress_sort = Status("Calling LM Studio for logical chronological course sequence...", spinner="dots", console=console)
        progress_sort.start()
        
        # Build payload of paths and metadata
        payload_meta = []
        for p in renamed_files:
            cache = MetadataBackupManager.load_cache(p)
            payload_meta.append({
                "path": str(p),
                "ai": cache.get("ai", {})
            })
            
        sorted_strings = VideoTaggerService.global_course_sort(payload_meta)
        # Reconstruct Path objects using exactly matched strings
        path_map = {str(p): p for p in renamed_files}
        sorted_files = [path_map[s] for s in sorted_strings if s in path_map]
        # Append any files the LLM missed so nothing is dropped
        sorted_set = set(sorted_strings)
        for p in renamed_files:
            if str(p) not in sorted_set:
                sorted_files.append(p)
        renamed_files = sorted_files
            
        progress_sort.stop()
        console.print(f"[green]✔ Logical LLM Course Reordering Complete ({len(renamed_files)} videos sequenced)[/green]\n")

        # ── Intermediate VRAM Flush ─────────────────────────────────────
        console.print("[cyan]Purging LM Studio from VRAM for NVENC Phase...[/cyan]")
        try:
            from aicli.config import unload_all_models
            unload_all_models()
        except Exception:
            pass

        # ════════════════════════════════════════════════════════════════
        # PHASE 3: Slideshow Compression (-w parallel FFmpeg workers)
        # ════════════════════════════════════════════════════════════════
        print_header(f"Phase 3: Slideshow Compression ({w3} workers)")
        slideshow_files = []
        with Progress(
            SpinnerColumn(), TextColumn("[progress.description]{task.description}"),
            BarColumn(), TaskProgressColumn(), TimeElapsedColumn(), console=console
        ) as progress:
            task_p3 = progress.add_task("GPU NVENC Fast-Skipping...", total=len(renamed_files))
            with ThreadPoolExecutor(max_workers=w3) as executor:
                futures = {}
                for f in renamed_files:
                    # Dynamically inject the cached AI tags and SRTs into the final Phase 3 compression!
                    cache = MetadataBackupManager.load_cache(f)
                    ai_tags = cache.get("ai", {})
                    target_name = ai_tags.get("filename", f.stem)
                    ext_srt = f.parent / ".aicli_cache" / f"{f.stem}.srt"
                    
                    slideshows_dir = f.parent / ".aicli_cache" / "slideshows"
                    slideshows_dir.mkdir(parents=True, exist_ok=True)
                    out_dest = slideshows_dir / f"{target_name}_slideshow.mp4"
                    
                    if out_dest.exists() and CompressService.get_file_size_mb(out_dest) > 0.1:
                        slideshow_files.append((out_dest, f))
                        progress.console.print(f"[dim]Already compressed → {out_dest.name}[/dim]")
                        progress.advance(task_p3)
                        continue
                    
                    fut = executor.submit(
                        CompressService.compress,
                        video_path=f,
                        output_path=out_dest,
                        resolution=0,
                        preset="slideshow",
                        fps="1/2",
                        fast_skip=True,
                        metadata_tags=ai_tags,
                        external_srt=ext_srt,
                        target_name=target_name
                    )
                    futures[fut] = f
                        
                for future in as_completed(futures):
                    try:
                        out_path = future.result()
                        slideshow_files.append((out_path, futures[future]))
                        progress.console.print(f"[green]Compressed → {out_path.name}[/green]")
                    except Exception as e:
                        progress.console.print(f"[red]Compression failed: {e}[/red]")
                    progress.advance(task_p3)
        # Reconstruct the exact slideshow ordering natively based on the LLM's sequence from Phase 2
        slideshow_files = []
        for f in renamed_files:
            cache = MetadataBackupManager.load_cache(f)
            target_name = cache.get("ai", {}).get("filename", f.stem)
            out_dest = f.parent / ".aicli_cache" / "slideshows" / f"{target_name}_slideshow.mp4"
            if out_dest.exists():
                slideshow_files.append((out_dest, f))

        # ════════════════════════════════════════════════════════════════
        # PHASE 4: Deep Native Merging
        # ════════════════════════════════════════════════════════════════
        cache_dir = target_dir / ".aicli_cache"
        
        # ── Chunking logic for max length ──────────
        max_sec = max_merge_hours * 3600 if max_merge_hours > 0 else float('inf')
        
        chunks = []
        current_chunk = []
        current_sec = 0

        from aicli.services.video.merge_service import MergeService
        for item in slideshow_files:
            out_dest, f = item
            try:
                dur = MergeService.get_video_duration(out_dest)
            except Exception:
                dur = 0
            
            if current_chunk and current_sec + dur > max_sec:
                chunks.append(current_chunk)
                current_chunk = []
                current_sec = 0
            
            current_chunk.append(item)
            current_sec += dur

        if current_chunk:
            chunks.append(current_chunk)

        is_multipart = len(chunks) > 1

        print_header(f"Phase 4: Deep Native Merging ({len(chunks)} parts)")

        merged_txts = []
        for i, chunk in enumerate(chunks, 1):
            if not chunk: continue
            part_suffix = f"_Part{i}" if is_multipart else ""
            
            # The user explicitly requested all final merged outputs to sit beautifully in the UI root folder
            merged_vid = target_dir / f"Course_Merged_Slideshow{part_suffix}.mp4"
            merged_vid_tmp = target_dir / f"Course_Merged_Slideshow_tmp{part_suffix}.mp4"
            merged_srt = target_dir / f"Course_Merged{part_suffix}.srt"
            merged_txt = target_dir / f"Course_Merged{part_suffix}.txt"
            merged_txts.append(merged_txt)

            if merged_vid.exists() and merged_srt.exists() and merged_txt.exists():
                console.print(f"[dim]Already merged → {merged_vid.name}[/dim]")
                continue

            # Generate perfectly synchronized master SRT first
            console.print(f"[cyan]Time-shifting and merging SRTs{(' (Part ' + str(i) + ')') if is_multipart else ''}...[/cyan]")
            video_srt_pairs = []
            for out_dest, orig_f in chunk:
                srt = cache_dir / f"{orig_f.stem}.srt"
                if not srt.exists():
                    srt = cache_dir / f"{orig_f.stem}.tmp_cc.srt"
                video_srt_pairs.append((out_dest, srt))
            if MergeService.merge_srts(video_srt_pairs, merged_srt):
                console.print(f"[bold green]✔ Saved {merged_srt.name}[/bold green]")

            # Now stitch video chunks losslessly and hard-mux the master SRT directly into the container!
            console.print(f"[cyan]Stitching videos and embedding master CC track{(' (Part ' + str(i) + ')') if is_multipart else ''}...[/cyan]")
            video_paths = [item[0] for item in chunk]
            if MergeService.merge_videos(video_paths, merged_vid_tmp):
                if merged_srt.exists():
                    import subprocess
                    subprocess.run([
                        "ffmpeg", "-y", "-v", "quiet",
                        "-i", str(merged_vid_tmp),
                        "-i", str(merged_srt),
                        "-map", "0:v:0", "-map", "1:s:0", # Slideshows don't have audio, just video and CC
                        "-c", "copy", "-c:s", "mov_text",
                        str(merged_vid)
                    ], capture_output=True)
                    if merged_vid.exists():
                        merged_vid_tmp.unlink(missing_ok=True)
                        console.print(f"[bold green]✔ Saved {merged_vid.name} (embedded CC natively inside video)[/bold green]")
                else:
                    merged_vid_tmp.rename(merged_vid)
                    console.print(f"[bold green]✔ Saved {merged_vid.name}[/bold green]")

            console.print(f"[cyan]Appending raw text transcripts{(' (Part ' + str(i) + ')') if is_multipart else ''}...[/cyan]")
            txt_files = [cache_dir / f"{orig_f.stem}.txt" for _, orig_f in chunk]
            if MergeService.merge_txts(txt_files, merged_txt):
                console.print(f"[bold green]✔ Saved {merged_txt.name}[/bold green]")

        # ════════════════════════════════════════════════════════════════
        # PHASE 5: LM Studio 'No Fluff' Notes (hot-swap to bigger model)
        # (Temporarily disabled as requested)
        # ════════════════════════════════════════════════════════════════
        if False:
            print_header("Phase 5: LM Studio 'No Fluff' Clean Transcription")
            
            # Load notes model once
            notes_loaded = False
            if merged_txts and notes_llm:
                try:
                    console.print(f"[cyan]Hot-swapping to heavier model: '{notes_llm}'...[/cyan]")
                    resolved_notes = resolve_dynamic_model(notes_llm)
                    aicli_config.model_name = resolved_notes
                    console.print(f"[green]✔ Loaded notes model: {resolved_notes}[/green]")
                    notes_loaded = True
                except Exception as e:
                    console.print(f"[dim]Could not load notes model ({e}). Using current model.[/dim]")
                    
            for i, merged_txt in enumerate(merged_txts, 1):
                if not merged_txt.exists(): continue
                
                part_suffix = f"_Part{i}" if is_multipart else ""
                merged_md  = target_dir / f"Course_Merged_NoFluff{part_suffix}.md"
                
                console.print(f"[cyan]Streaming merged transcript{(' (Part ' + str(i) + ')') if is_multipart else ''} through LM Studio...[/cyan]")
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

        # ── Global VRAM Flush ───────────────────────────────────────────
        console.print("[cyan]Flushing all models from VRAM...[/cyan]")
        from aicli.config import unload_all_models
        unload_all_models()
        try:
            import torch, gc
            gc.collect()
            if torch.cuda.is_available():
                torch.cuda.empty_cache()
        except Exception:
            pass

        console.print("\n[bold magenta]===== GOD-MODE PIPELINE COMPLETE =====[/bold magenta]")
