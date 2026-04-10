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
        generate_thumb: bool, 
        retranscribe: bool, 
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
                progress.console.print(f"[dim]\[{video_path.name}] Loaded cached transcript[/dim]")
            else:
                progress.console.print(f"[cyan]\[{video_path.name}] Transcribing audio...[/cyan]")
                    
                clips = WhisperEngine.transcribe_video(video_path, whisper_model, clip_every, clip_len)
                if not clips:
                    return video_path, None, ValueError("No speech detected.")
                    
                cache["clips"] = clips
                MetadataBackupManager.save_cache(video_path, cache)

            if "ai" in cache and not retranscribe:
                ai = cache["ai"]
                progress.console.print(f"[dim]\[{video_path.name}] Loaded cached AI tags[/dim]")
            else:
                progress.console.print(f"[cyan]\[{video_path.name}] Requesting metadata from LM Studio...[/cyan]")
                ai = VideoTaggerService.ask_lmstudio(clips, str(video_path.parent))
                if not ai:
                    return video_path, None, ValueError("LM Studio returned empty response.")
                    
                cache["ai"] = ai
                MetadataBackupManager.save_cache(video_path, cache)

            progress.console.print(f"[green]\[{video_path.name}] Evaluated: {ai.get('title')} ({ai.get('subject')})[/green]")

            new_tags = {
                "title":       ai.get("title", ""),
                "comment":     ai.get("description", ""),
                "genre":       ai.get("subject", ""),
                "description": ai.get("description", ""),
                "SUBJECT":     ai.get("subject", ""),
                "TOPICS":      ", ".join(ai.get("topics", [])),
                "language_track": ai.get("language", ""),
                "SUMMARY":     ai.get("description", ""),
            }

            if write:
                FFmpegClient.write_tags(video_path, new_tags, original_tags=cache.get("original_tags"))
                progress.console.print(f"[{video_path.name}] [bold green]Tags and backup embedded natively.[/bold green]")

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

                if generate_thumb:
                    thumb_path = video_path.with_suffix(".jpg")
                    from aicli.services.video.ffmpeg_utils import FFprobeClient
                    duration = FFprobeClient.get_duration(video_path)
                    if FFmpegClient.generate_thumbnail(video_path, thumb_path, duration):
                        progress.console.print(f"[{video_path.name}] [bold magenta]Thumbnail generated: {thumb_path.name}[/bold magenta]")

            return video_path, ai, None
            
        except Exception as e:
            return video_path, None, e
