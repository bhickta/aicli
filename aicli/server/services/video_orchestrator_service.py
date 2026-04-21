import shutil
import subprocess
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

from rich.progress import (
    Progress,
    SpinnerColumn,
    TextColumn,
    BarColumn,
    TaskProgressColumn,
    TimeElapsedColumn,
)
from rich.status import Status

from aicli.cli.tui import print_header, console
from aicli.server.pipelines.video_processor import VideoBatchProcessor
from aicli.services.video.compress_service import CompressService
from aicli.services.video.merge_service import MergeService
from aicli.services.video.notes_service import NotesService
from aicli.services.video.tagger_service import VideoTaggerService
from aicli.services.video.transcriber import WhisperEngine
from aicli.services.video.metadata_manager import MetadataBackupManager
from aicli.config import config as aicli_config


class VideoOrchestratorService:
    @staticmethod
    def run_phase1_transcribe(
        needs_transcription: list[Path], whisper_model: str, workers: int
    ):
        if not needs_transcription:
            console.print("[green]All files cached. Skipping Whisper entirely.[/green]")
            return

        try:
            console.print(
                f"[cyan]Loading Whisper model on GPU ({whisper_model})...[/cyan]"
            )
            model_instance = WhisperEngine.load_whisper(
                whisper_model, num_workers=workers
            )
        except Exception as e:
            console.print(f"[red]Failed to load Whisper model: {e}[/red]")
            return

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TaskProgressColumn(),
            TimeElapsedColumn(),
            console=console,
        ) as progress:
            task_p1 = progress.add_task(
                f"Transcribing ({workers} GPU workers)...",
                total=len(needs_transcription),
            )
            with ThreadPoolExecutor(max_workers=workers) as executor:
                futures = {
                    executor.submit(
                        VideoBatchProcessor.phase1_transcribe,
                        f,
                        model_instance,
                        write=True,
                        full_cc=True,
                        retranscribe=False,
                        transcribe_only=False,
                        clip_every=0,
                        clip_len=0,
                        save_txt=True,
                        progress=progress,
                        task_id=task_p1,
                    ): f
                    for f in needs_transcription
                }
                for future in as_completed(futures):
                    path, err = future.result()
                    if err:
                        progress.console.print(
                            f"[red]Error transcribing {futures[future].name}: {err}[/red]"
                        )
                    progress.advance(task_p1)

        console.print("[cyan]Purging Whisper from VRAM...[/cyan]")
        try:
            del model_instance
            import torch, gc

            gc.collect()
            if torch.cuda.is_available():
                torch.cuda.empty_cache()
        except Exception:
            pass

    @staticmethod
    def run_phase2_tag_and_sort(
        raw_files: list[Path], llm_model: str, workers: int
    ) -> list[Path]:
        from aicli.providers import get_provider
        try:
            aicli_config.model_name = llm_model
            provider = get_provider()
            p_name = provider.__class__.__name__.replace("Provider", "")
            
            console.print(f"[cyan]Using {p_name} model: {llm_model}...[/cyan]")
            
            # Pre-check: try a dummy call to see if the model is actually loaded
            # This prevents 12 workers from all failing simultaneously with 400 errors
            try:
                # Minimal check
                from langchain_core.messages import HumanMessage
                if hasattr(provider, 'llm'):
                    provider.llm.invoke([HumanMessage(content="hi")], config={"timeout": 5})
                console.print(f"[green]✔ {p_name} model ready: {llm_model}[/green]")
            except Exception as e:
                console.print(f"[bold yellow]⚠️ Warning: {p_name} model '{llm_model}' might not be loaded or reachable.[/bold yellow]")
                console.print(f"[dim]Detail: {e}[/dim]")
                if "No models loaded" in str(e):
                    console.print("[bold red]ERROR: LM Studio has no model loaded. Please load a model in LM Studio GUI or via 'lms load'.[/bold red]")
        except Exception as e:
            console.print(
                f"[dim]Note: Could not verify LLM provider ({e}). Continuing.[/dim]"
            )

        renamed_files = []
        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TaskProgressColumn(),
            TimeElapsedColumn(),
            console=console,
        ) as progress:
            task_p2 = progress.add_task("Tagging and Renaming...", total=len(raw_files))
            with ThreadPoolExecutor(max_workers=workers) as executor:
                futures = {
                    executor.submit(
                        VideoBatchProcessor.phase2_tag_and_mux,
                        f,
                        write=True,
                        no_rename=False,
                        text_thumb=False,
                        retranscribe=False,
                        transcribe_only=False,
                        progress=progress,
                        task_id=task_p2,
                    ): f
                    for f in raw_files
                }
                for future in as_completed(futures):
                    new_path, _, err = future.result()
                    if err:
                        progress.console.print(
                            f"[red]Error tagging {futures[future].name}: {err}[/red]"
                        )
                        renamed_files.append(futures[future])
                    else:
                        renamed_files.append(new_path)
                    progress.advance(task_p2)

        from aicli.providers import get_provider
        provider = get_provider()
        p_name = provider.__class__.__name__.replace("Provider", "")
        
        progress_sort = Status(
            f"Calling {p_name} for logical chronological course sequence...",
            spinner="dots",
            console=console,
        )
        progress_sort.start()

        payload_meta = [
            {"path": str(p), "ai": MetadataBackupManager.load_cache(p).get("ai", {})}
            for p in renamed_files
        ]
        sorted_strings = VideoTaggerService.global_course_sort(payload_meta)

        path_map = {str(p): p for p in renamed_files}
        sorted_files = [path_map[s] for s in sorted_strings if s in path_map]

        sorted_set = set(sorted_strings)
        for p in renamed_files:
            if str(p) not in sorted_set:
                sorted_files.append(p)
        renamed_files = sorted_files

        progress_sort.stop()
        console.print(
            f"[green]✔ Logical LLM Course Reordering Complete ({len(renamed_files)} videos sequenced)[/green]\n"
        )

        # Purge LLM from VRAM before Phase 3 (Compression)
        try:
            import torch, gc, time
            gc.collect()
            if torch.cuda.is_available():
                torch.cuda.empty_cache()
            
            # If using LM Studio, tell it to unload via CLI
            target_model = llm_model if llm_model else aicli_config.model_name
            if aicli_config.provider_type == "lmstudio" and shutil.which("lms"):
                console.print(f"[dim]LM Studio: Unloading '{target_model}'...[/dim]")
                subprocess.run(["lms", "unload", target_model], capture_output=True)
                time.sleep(2) # Give GPU driver time to clear VRAM
        except Exception:
            pass

        console.print("[cyan]NVENC compression phase...[/cyan]")

        return renamed_files

    @staticmethod
    def run_phase3_compress(
        renamed_files: list[Path], workers: int
    ) -> list[tuple[Path, Path]]:
        # Hard cap for NVENC stability on consumer GPUs (usually limited to 5-8 sessions)
        workers = min(workers, 4)
        slideshow_files = []
        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TaskProgressColumn(),
            TimeElapsedColumn(),
            console=console,
        ) as progress:
            task_p3 = progress.add_task(
                "GPU NVENC Fast-Skipping...", total=len(renamed_files)
            )
            with ThreadPoolExecutor(max_workers=workers) as executor:
                futures = {}
                for f in renamed_files:
                    cache = MetadataBackupManager.load_cache(f)
                    ai_tags = cache.get("ai", {})
                    raw_target = ai_tags.get("filename", f.stem)
                    # Sanitize filename: remove colons, slashes, and other illegal characters
                    target_name = "".join(c for c in raw_target if c.isalnum() or c in (" ", ".", "_", "-")).strip()
                    if not target_name:
                        target_name = f.stem
                    ext_srt = f.parent / ".aicli_cache" / f"{f.stem}.srt"

                    slideshows_dir = f.parent / ".aicli_cache" / "slideshows"
                    slideshows_dir.mkdir(parents=True, exist_ok=True)
                    out_dest = slideshows_dir / f"{target_name}_slideshow.mp4"

                    if (
                        out_dest.exists()
                        and CompressService.get_file_size_mb(out_dest) > 0.1
                    ):
                        slideshow_files.append((out_dest, f))
                        progress.console.print(
                            f"[dim]Already compressed → {out_dest.name}[/dim]"
                        )
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
                        target_name=target_name,
                    )
                    futures[fut] = f

                for future in as_completed(futures):
                    try:
                        out_path = future.result()
                        slideshow_files.append((out_path, futures[future]))
                        progress.console.print(
                            f"[green]Compressed → {out_path.name}[/green]"
                        )
                    except Exception as e:
                        progress.console.print(f"[red]Compression failed: {e}[/red]")
                    progress.advance(task_p3)

        # Restore strict LLM sorted order
        ordered_slideshows = []
        for f in renamed_files:
            cache = MetadataBackupManager.load_cache(f)
            raw_target = cache.get("ai", {}).get("filename", f.stem)
            target_name = "".join(c for c in raw_target if c.isalnum() or c in (" ", ".", "_", "-")).strip()
            if not target_name:
                target_name = f.stem
                
            out_dest = (
                f.parent
                / ".aicli_cache"
                / "slideshows"
                / f"{target_name}_slideshow.mp4"
            )
            if out_dest.exists():
                ordered_slideshows.append((out_dest, f))

        return ordered_slideshows

    @staticmethod
    def run_phase4_merge(
        target_dir: Path,
        slideshow_files: list[tuple[Path, Path]],
        max_merge_hours: float,
    ):
        cache_dir = target_dir / ".aicli_cache"
        max_sec = max_merge_hours * 3600 if max_merge_hours > 0 else float("inf")

        chunks = []
        current_chunk = []
        current_sec = 0

        for item in slideshow_files:
            out_dest, f = item
            try:
                # We calculate expected duration from the ORIGINAL source to ensure 100% integrity
                dur = MergeService.get_video_duration(f)
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

        for i, chunk in enumerate(chunks, 1):
            if not chunk:
                continue
            part_suffix = f"_Part{i}" if is_multipart else ""

            merged_vid = target_dir / f"Course_Merged_Slideshow{part_suffix}.mp4"
            merged_vid_tmp = (
                target_dir / f"Course_Merged_Slideshow_tmp{part_suffix}.mp4"
            )
            merged_srt = target_dir / f"Course_Merged{part_suffix}.srt"
            merged_txt = target_dir / f"Course_Merged{part_suffix}.txt"

            if merged_vid.exists() and merged_srt.exists() and merged_txt.exists():
                console.print(f"[dim]Already merged → {merged_vid.name}[/dim]")
                continue

            console.print(
                f"[cyan]Time-shifting and merging SRTs{(' (Part ' + str(i) + ')') if is_multipart else ''}...[/cyan]"
            )
            video_srt_pairs = []
            for out_dest, orig_f in chunk:
                srt = cache_dir / f"{orig_f.stem}.srt"
                if not srt.exists():
                    srt = cache_dir / f"{orig_f.stem}.tmp_cc.srt"
                video_srt_pairs.append((out_dest, srt))
            if MergeService.merge_srts(video_srt_pairs, merged_srt):
                console.print(f"[bold green]✔ Saved {merged_srt.name}[/bold green]")

            console.print(
                f"[cyan]Stitching videos and embedding master CC track{(' (Part ' + str(i) + ')') if is_multipart else ''}...[/cyan]"
            )
            video_paths = [item[0] for item in chunk]
            if MergeService.merge_videos(video_paths, merged_vid_tmp):
                if merged_srt.exists():
                    MergeService.embed_srt_natively(
                        merged_vid_tmp, merged_srt, merged_vid
                    )
                    console.print(
                        f"[bold green]✔ Saved {merged_vid.name} (embedded CC natively inside video)[/bold green]"
                    )
                else:
                    merged_vid_tmp.rename(merged_vid)
                    console.print(f"[bold green]✔ Saved {merged_vid.name}[/bold green]")

                # Sanity Check: Verify duration
                try:
                    final_dur = MergeService.get_video_duration(merged_vid)
                    diff = abs(final_dur - current_sec)
                    if diff > 1.0: # Allow 1 second tolerance for container overhead
                        console.print(f"[bold yellow]⚠️ Warning: Duration mismatch in {merged_vid.name}. Expected {current_sec:.2f}s, got {final_dur:.2f}s (diff: {diff:.2f}s)[/bold yellow]")
                    else:
                        console.print(f"[green]✔ Duration verified: {final_dur:.2f}s matches parts sum.[/green]")
                except Exception:
                    pass

            console.print(
                f"[cyan]Appending raw text transcripts{(' (Part ' + str(i) + ')') if is_multipart else ''}...[/cyan]"
            )
            txt_files = [cache_dir / f"{orig_f.stem}.txt" for _, orig_f in chunk]
            if MergeService.merge_txts(txt_files, merged_txt):
                console.print(f"[bold green]✔ Saved {merged_txt.name}[/bold green]")

    @staticmethod
    def run_phase6_cleanup(target_dir: Path, slideshow_files: list[tuple[Path, Path]]):
        trash_dir = target_dir / "Trash"
        trash_dir.mkdir(exist_ok=True)
        count = 0
        cache_dir = target_dir / ".aicli_cache"
        for f, original in slideshow_files:
            base = original.stem
            srt_f = cache_dir / f"{base}.srt"
            txt_f = cache_dir / f"{base}.txt"
            if f.exists():
                shutil.move(str(f), str(trash_dir / f.name))
                count += 1
            if srt_f.exists():
                shutil.move(str(srt_f), str(trash_dir / srt_f.name))
                count += 1
            if txt_f.exists():
                shutil.move(str(txt_f), str(trash_dir / txt_f.name))
                count += 1
        console.print(f"[dim]Moved {count} intermediate files to Trash folder.[/dim]")
