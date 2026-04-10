from pathlib import Path
import shutil

from rich.progress import Progress

from aicli.services.video.ffmpeg_utils import FFmpegClient
from aicli.services.video.transcriber import WhisperEngine
from aicli.services.video.metadata_manager import MetadataBackupManager
from aicli.services.video.tagger_service import VideoTaggerService

class VideoBatchProcessor:
    """Handles the sequential processing pipeline mapped to UI output."""
    
    @staticmethod
    def process_isolated_video(
        video_path: Path, 
        whisper_model, 
        write: bool, 
        no_rename: bool, 
        full_cc: bool,
        text_thumb: bool,
        retranscribe: bool,
        transcribe_only: bool,
        clip_every: int, 
        clip_len: int, 
        progress: Progress, 
        task_id
    ) -> tuple[Path, dict, Exception]:
        """Workflow sequence to process a single video inside a thread executor."""
        try:
            cache = MetadataBackupManager.load_cache(video_path) if not retranscribe else {}
            MetadataBackupManager.backup_original_tags(video_path, cache)
            MetadataBackupManager.save_cache(video_path, cache)

            if "clips" in cache and not retranscribe:
                clips = cache["clips"]
                progress.console.print(f"[dim]\\[{video_path.name}] Loaded cached transcript[/dim]")
            else:
                # We always generate a temporary .srt if --full-cc is requested so we can mux it later
                srt_path = video_path.with_suffix(".tmp_cc.srt") 
                
                # Check if a non-temp SRT exists to salvage text context from
                ext_srt = video_path.with_suffix(".srt")
                if ext_srt.exists() and not retranscribe:
                    progress.console.print(f"[green]\\[{video_path.name}] Reading context directly from existing .srt...[/green]")
                    clips = WhisperEngine.extract_clips_from_existing_srt(ext_srt)
                    if full_cc: 
                        shutil.copy(ext_srt, srt_path) # Stage for muxing
                elif full_cc:
                    progress.console.print(f"[purple]\\[{video_path.name}] Fully Transcribing to container CCs...[/purple]")
                    clips = WhisperEngine.transcribe_video_full_srt(video_path, whisper_model, srt_path)
                else:
                    progress.console.print(f"[cyan]\\[{video_path.name}] Extracting sparse transcript samples...[/cyan]")
                    clips = WhisperEngine.transcribe_video_sparse(video_path, whisper_model, clip_every, clip_len)
                    
                if not clips:
                    return video_path, None, ValueError("No speech or text detected.")
                    
                cache["clips"] = clips
                MetadataBackupManager.save_cache(video_path, cache)

            if transcribe_only:
                tmp_srt = video_path.with_suffix(".tmp_cc.srt")
                
                if write and full_cc and tmp_srt.exists():
                    # Mux the SRT directly into the video container — no tagging, no renaming
                    FFmpegClient.write_tags(video_path, {}, srt_path=tmp_srt)
                    if tmp_srt.exists():
                        tmp_srt.unlink()
                    progress.console.print(f"[bold green]\\[{video_path.name}] CC track embedded into container.[/bold green]")
                elif tmp_srt.exists():
                    # No --write, just save the .srt as a sidecar
                    final_srt = video_path.with_suffix(".srt")
                    shutil.move(str(tmp_srt), str(final_srt))
                    progress.console.print(f"[bold green]\\[{video_path.name}] Saved transcript to {final_srt.name}[/bold green]")
                else:
                    progress.console.print(f"[bold green]\\[{video_path.name}] Transcript cached (use --full-cc to generate SRT).[/bold green]")
                return video_path, {}, None

            if "ai" in cache and not retranscribe:
                ai = cache["ai"]
                progress.console.print(f"[dim]\\[{video_path.name}] Loaded cached AI tags[/dim]")
            else:
                progress.console.print(f"[cyan]\\[{video_path.name}] Requesting metadata from LM Studio...[/cyan]")
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
                tmp_srt = video_path.with_suffix(".tmp_cc.srt")
                embed_path = tmp_srt if tmp_srt.exists() else None
                
                tmp_cover = None
                if text_thumb:
                    tmp_cover = video_path.with_suffix(".tmp_cover.jpg")
                    if not FFmpegClient.generate_text_thumbnail(ai.get("title", video_path.name), tmp_cover):
                        tmp_cover = None
                        
                FFmpegClient.write_tags(video_path, new_tags, original_tags=cache.get("original_tags"), srt_path=embed_path, cover_path=tmp_cover)
                
                if embed_path and tmp_cover:
                    progress.console.print(f"[{video_path.name}] [bold green]Tags, text cover-art, and CC track embedded natively.[/bold green]")
                else:
                    progress.console.print(f"[{video_path.name}] [bold green]Tags embedded natively into container.[/bold green]")

                # Delete temporary tracks
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
