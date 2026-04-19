from pathlib import Path
import shutil

from rich.progress import Progress

from aicli.services.video.ffmpeg_utils import FFmpegClient, FFprobeClient
from aicli.services.video.transcriber import WhisperEngine
from aicli.services.video.metadata_manager import MetadataBackupManager
from aicli.services.video.tagger_service import VideoTaggerService
from aicli.services.video.notes_service import NotesService


class VideoBatchProcessor:
    """Handles the sequential processing pipeline mapped to UI output."""

    @staticmethod
    def phase1_transcribe(
        video_path: Path,
        whisper_model,
        write: bool,
        full_cc: bool,
        retranscribe: bool,
        transcribe_only: bool,
        clip_every: int,
        clip_len: int,
        save_txt: bool,
        progress: Progress,
        task_id,
    ) -> tuple[Path, Exception]:
        """Workflow sequence to ONLY transcribe a single video."""
        try:
            cache_dir = video_path.parent / ".aicli_cache"
            cache_dir.mkdir(exist_ok=True, parents=True)

            cache = (
                MetadataBackupManager.load_cache(video_path) if not retranscribe else {}
            )
            MetadataBackupManager.backup_original_tags(video_path, cache)
            MetadataBackupManager.save_cache(video_path, cache)

            if "clips" in cache and not retranscribe:
                progress.console.print(
                    f"[dim]\\[{video_path.name}] Loaded cached transcript[/dim]"
                )
            else:
                srt_path = cache_dir / f"{video_path.stem}.tmp_cc.srt"

                ext_srt = cache_dir / f"{video_path.stem}.srt"
                # Also check root-level for legacy SRTs from previous runs
                if not ext_srt.exists():
                    legacy_srt = video_path.with_suffix(".srt")
                    if legacy_srt.exists():
                        shutil.copy(str(legacy_srt), str(ext_srt))
                if ext_srt.exists() and not retranscribe:
                    progress.console.print(
                        f"[green]\\[{video_path.name}] Reading context directly from existing .srt...[/green]"
                    )
                    clips = WhisperEngine.extract_clips_from_existing_srt(ext_srt)
                    if full_cc:
                        shutil.copy(ext_srt, srt_path)
                elif full_cc:
                    progress.console.print(
                        f"[purple]\\[{video_path.name}] Fully Transcribing to container CCs...[/purple]"
                    )
                    clips = WhisperEngine.transcribe_video_full_srt(
                        video_path, whisper_model, srt_path
                    )
                else:
                    progress.console.print(
                        f"[cyan]\\[{video_path.name}] Extracting sparse transcript samples...[/cyan]"
                    )
                    clips = WhisperEngine.transcribe_video_sparse(
                        video_path, whisper_model, clip_every, clip_len
                    )

                if not clips:
                    return video_path, ValueError("No speech or text detected.")

                cache["clips"] = clips
                MetadataBackupManager.save_cache(video_path, cache)

            # Handle .txt export if requested
            if save_txt:
                src_srt = cache_dir / f"{video_path.stem}.tmp_cc.srt"
                if not src_srt.exists():
                    src_srt = cache_dir / f"{video_path.stem}.srt"

                if src_srt.exists():
                    try:
                        clean_text = NotesService.srt_to_text(src_srt)
                        (cache_dir / f"{video_path.stem}.txt").write_text(
                            clean_text, encoding="utf-8"
                        )
                        progress.console.print(
                            f"[bold green]\\[{video_path.name}] Clean transcript saved to .txt[/bold green]"
                        )
                    except Exception as e:
                        progress.console.print(
                            f"[red]\\[{video_path.name}] Failed to save .txt transcript: {e}[/red]"
                        )

            if transcribe_only:
                tmp_srt = cache_dir / f"{video_path.stem}.tmp_cc.srt"
                if write and full_cc and tmp_srt.exists():
                    FFmpegClient.write_tags(video_path, {}, srt_path=tmp_srt)
                    if tmp_srt.exists():
                        tmp_srt.unlink()
                    progress.console.print(
                        f"[bold green]\\[{video_path.name}] CC track embedded into container.[/bold green]"
                    )
                elif tmp_srt.exists():
                    final_srt = cache_dir / f"{video_path.stem}.srt"
                    shutil.move(str(tmp_srt), str(final_srt))
                    progress.console.print(
                        f"[bold green]\\[{video_path.name}] Saved transcript to {final_srt.name}[/bold green]"
                    )
                else:
                    progress.console.print(
                        f"[bold green]\\[{video_path.name}] Transcript cached (use --full-cc to generate SRT).[/bold green]"
                    )

            return video_path, None

        except Exception as e:
            return video_path, e

    @staticmethod
    def phase2_tag_and_mux(
        video_path: Path,
        write: bool,
        no_rename: bool,
        text_thumb: bool,
        retranscribe: bool,
        transcribe_only: bool,
        progress: Progress,
        task_id,
    ) -> tuple[Path, dict, Exception]:
        """Workflow sequence to strictly use LM Studio and Mux tags into the container."""
        if transcribe_only:
            return video_path, {}, None

        try:
            cache_dir = video_path.parent / ".aicli_cache"
            cache_dir.mkdir(exist_ok=True, parents=True)

            cache = MetadataBackupManager.load_cache(video_path)

            if "ai" in cache and not retranscribe:
                ai = cache["ai"]
                progress.console.print(
                    f"[dim]\\[{video_path.name}] Loaded cached AI tags[/dim]"
                )
            else:
                # Muted: progress.console.print(f"[cyan]\\[{video_path.name}] Requesting metadata from LM Studio...[/cyan]")

                clips = cache.get("clips", [])
                if not clips:
                    # Recovery: try to reconstruct clips from existing .srt or .txt in cache dir
                    srt_path = cache_dir / f"{video_path.stem}.srt"
                    txt_path = cache_dir / f"{video_path.stem}.txt"
                    if srt_path.exists():
                        raw = srt_path.read_text(encoding="utf-8", errors="replace")
                        import re as _re

                        blocks = _re.split(r"\n\n+", raw.strip())
                        for block in blocks:
                            lines = block.strip().split("\n")
                            if len(lines) >= 3:
                                text = " ".join(lines[2:])
                                clips.append({"start_sec": 0, "text": text})
                    elif txt_path.exists():
                        raw = txt_path.read_text(encoding="utf-8", errors="replace")
                        clips = [{"start_sec": 0, "text": raw[:4000]}]
                    else:
                        # Last resort: extract embedded subtitle stream via ffmpeg
                        import subprocess, tempfile

                        with tempfile.NamedTemporaryFile(
                            suffix=".srt", delete=False
                        ) as tmp:
                            tmp_extract = tmp.name
                        try:
                            subprocess.run(
                                [
                                    "ffmpeg",
                                    "-y",
                                    "-v",
                                    "quiet",
                                    "-i",
                                    str(video_path),
                                    "-map",
                                    "0:s:0",
                                    "-c:s",
                                    "srt",
                                    tmp_extract,
                                ],
                                capture_output=True,
                                timeout=30,
                            )
                            from pathlib import Path as _P

                            tmp_p = _P(tmp_extract)
                            if tmp_p.exists() and tmp_p.stat().st_size > 10:
                                raw = tmp_p.read_text(
                                    encoding="utf-8", errors="replace"
                                )
                                import re as _re

                                blocks = _re.split(r"\n\n+", raw.strip())
                                for block in blocks:
                                    lines = block.strip().split("\n")
                                    if len(lines) >= 3:
                                        text = " ".join(lines[2:])
                                        clips.append({"start_sec": 0, "text": text})
                            tmp_p.unlink(missing_ok=True)
                        except Exception:
                            pass

                    if not clips:
                        # Ultimate fallback: use the filename itself as context
                        # Filenames like "LESSON_26_UNIT_2_Nuclear_Diplomacy" are descriptive enough
                        clean_name = video_path.stem.replace("_", " ").replace("-", " ")
                        clips = [
                            {"start_sec": 0, "text": f"Lecture video: {clean_name}"}
                        ]

                ai = VideoTaggerService.ask_ollama(clips, str(video_path.parent))
                if not ai:
                    return (
                        video_path,
                        None,
                        ValueError("LM Studio returned empty response."),
                    )

                cache["ai"] = ai
                MetadataBackupManager.save_cache(video_path, cache)

            # Muted console spam for cleaner logs
            # progress.console.print(f"[green]\\[{video_path.name}] Evaluated: {ai.get('title')} ({ai.get('subject')})[/green]")

            # In zero-copy God-Mode, we do NOT embed tags or rename the file at this stage.
            # We strictly return the generated tags to be natively injected during Phase 3 NVENC compression.
            return video_path, ai, None

        except Exception as e:
            return video_path, None, e
