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
        task_id
    ) -> tuple[Path, Exception]:
        """Workflow sequence to ONLY transcribe a single video."""
        try:
            cache_dir = video_path.parent / ".aicli_cache"
            cache_dir.mkdir(exist_ok=True, parents=True)
            
            cache = MetadataBackupManager.load_cache(video_path) if not retranscribe else {}
            MetadataBackupManager.backup_original_tags(video_path, cache)
            MetadataBackupManager.save_cache(video_path, cache)

            if "clips" in cache and not retranscribe:
                progress.console.print(f"[dim]\\[{video_path.name}] Loaded cached transcript[/dim]")
            else:
                srt_path = cache_dir / f"{video_path.stem}.tmp_cc.srt"
                
                ext_srt = cache_dir / f"{video_path.stem}.srt"
                # Also check root-level for legacy SRTs from previous runs
                if not ext_srt.exists():
                    legacy_srt = video_path.with_suffix(".srt")
                    if legacy_srt.exists():
                        shutil.copy(str(legacy_srt), str(ext_srt))
                if ext_srt.exists() and not retranscribe:
                    progress.console.print(f"[green]\\[{video_path.name}] Reading context directly from existing .srt...[/green]")
                    clips = WhisperEngine.extract_clips_from_existing_srt(ext_srt)
                    if full_cc: 
                        shutil.copy(ext_srt, srt_path) 
                elif full_cc:
                    progress.console.print(f"[purple]\\[{video_path.name}] Fully Transcribing to container CCs...[/purple]")
                    clips = WhisperEngine.transcribe_video_full_srt(video_path, whisper_model, srt_path)
                else:
                    progress.console.print(f"[cyan]\\[{video_path.name}] Extracting sparse transcript samples...[/cyan]")
                    clips = WhisperEngine.transcribe_video_sparse(video_path, whisper_model, clip_every, clip_len)
                    
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
                        (cache_dir / f"{video_path.stem}.txt").write_text(clean_text, encoding="utf-8")
                        progress.console.print(f"[bold green]\\[{video_path.name}] Clean transcript saved to .txt[/bold green]")
                    except Exception as e:
                        progress.console.print(f"[red]\\[{video_path.name}] Failed to save .txt transcript: {e}[/red]")

            if transcribe_only:
                tmp_srt = cache_dir / f"{video_path.stem}.tmp_cc.srt"
                if write and full_cc and tmp_srt.exists():
                    FFmpegClient.write_tags(video_path, {}, srt_path=tmp_srt)
                    if tmp_srt.exists(): tmp_srt.unlink()
                    progress.console.print(f"[bold green]\\[{video_path.name}] CC track embedded into container.[/bold green]")
                elif tmp_srt.exists():
                    final_srt = cache_dir / f"{video_path.stem}.srt"
                    shutil.move(str(tmp_srt), str(final_srt))
                    progress.console.print(f"[bold green]\\[{video_path.name}] Saved transcript to {final_srt.name}[/bold green]")
                else:
                    progress.console.print(f"[bold green]\\[{video_path.name}] Transcript cached (use --full-cc to generate SRT).[/bold green]")

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
        task_id
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
                progress.console.print(f"[dim]\\[{video_path.name}] Loaded cached AI tags[/dim]")
            else:
                progress.console.print(f"[cyan]\\[{video_path.name}] Requesting metadata from LM Studio...[/cyan]")
                
                clips = cache.get("clips", [])
                if not clips:
                    return video_path, None, ValueError("No transcript cache found to tag.")
                    
                ai = VideoTaggerService.ask_lmstudio(clips, str(video_path.parent))
                if not ai:
                    return video_path, None, ValueError("LM Studio returned empty response.")
                    
                cache["ai"] = ai
                MetadataBackupManager.save_cache(video_path, cache)

            progress.console.print(f"[green]\\[{video_path.name}] Evaluated: {ai.get('title')} ({ai.get('subject')})[/green]")

            new_tags = {
                "title":       ai.get("title", ""),
                "comment":     ai.get("description", ""),
                "genre":       ai.get("subject", ""),
                "description": ai.get("description", ""),
                "artist":      ai.get("teacher", ""),
                "publisher":   ai.get("coaching", ""),
                "SUBJECT":     ai.get("subject", ""),
                "TOPICS":      ", ".join(ai.get("topics", [])),
                "language_track": ai.get("language", ""),
                "SUMMARY":     ai.get("description", ""),
            }

            if write:
                tmp_srt = cache_dir / f"{video_path.stem}.tmp_cc.srt"
                # Skip SRT embedding if the container already has subtitles from a previous run
                if tmp_srt.exists() and not FFprobeClient.has_subtitle_stream(video_path):
                    embed_path = tmp_srt
                else:
                    embed_path = None
                
                tmp_cover = None
                if text_thumb:
                    tmp_cover = cache_dir / f"{video_path.stem}.tmp_cover.jpg"
                    if not FFmpegClient.generate_text_thumbnail(ai.get("title", video_path.name), tmp_cover):
                        tmp_cover = None
                        
                FFmpegClient.write_tags(video_path, new_tags, original_tags=cache.get("original_tags"), srt_path=embed_path, cover_path=tmp_cover)
                
                if embed_path and tmp_cover:
                    progress.console.print(f"[{video_path.name}] [bold green]Tags, text cover-art, and CC track embedded natively.[/bold green]")
                else:
                    progress.console.print(f"[{video_path.name}] [bold green]Tags embedded natively into container.[/bold green]")

                if embed_path and embed_path.exists(): embed_path.unlink()
                if tmp_cover and tmp_cover.exists(): tmp_cover.unlink()

                sc_path = MetadataBackupManager.sidecar_path(video_path)
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
                            video_path = new_path

            return video_path, ai, None
            
        except Exception as e:
            return video_path, None, e
