from pathlib import Path
from rich.table import Table

from aicli.services.video.ffprobe import FFprobeClient
from aicli.services.video.metadata_manager import MetadataBackupManager
from aicli.services.video.tagger_service import VideoTaggerService
from aicli.services.video.notes_service import NotesService
from aicli.cli.tui import console

class VideoCacheRepository:
    @staticmethod
    def get_raw_files(target_dir: Path) -> list[Path]:
        valid_exts = VideoTaggerService.VIDEO_EXTENSIONS
        return [
            p for p in target_dir.rglob("*")
            if p.is_file() and p.suffix.lower() in valid_exts
            and ".aicli_cache" not in str(p)
            and "slideshow" not in p.name.lower()
            and "merged" not in p.name.lower()
        ]

    @staticmethod
    def evaluate_pipeline_state(raw_files: list[Path], target_dir: Path) -> dict:
        import subprocess

        state = {
            "needs_transcription": [],
            "already_done": [],
            "needs_tagging": 0,
            "needs_rename": 0,
            "needs_compress": 0,
            "raw_files": raw_files
        }

        for f in raw_files:
            cache = MetadataBackupManager.load_cache(f)
            has_cache = "clips" in cache
            has_subs = FFprobeClient.has_subtitle_stream(f)
            
            ext_srt = f.parent / ".aicli_cache" / f"{f.stem}.srt"
            ext_txt = f.parent / ".aicli_cache" / f"{f.stem}.txt"
            
            if has_subs and not ext_srt.exists():
                ext_srt.parent.mkdir(parents=True, exist_ok=True)
                subprocess.run(["ffmpeg", "-y", "-v", "quiet", "-i", str(f), "-map", "0:s:0", "-c:s", "srt", str(ext_srt)])
                
            if ext_srt.exists() and not ext_txt.exists():
                try:
                    clean_text = NotesService.srt_to_text(ext_srt)
                    ext_txt.write_text(clean_text, encoding="utf-8")
                except Exception:
                    ext_txt.write_text("", encoding="utf-8")
            
            if (has_cache or has_subs) and ext_srt.exists() and ext_txt.exists():
                state["already_done"].append(f)
            else:
                state["needs_transcription"].append(f)
            
            if "ai" not in cache:
                state["needs_tagging"] += 1
            if "original_filename" not in cache:
                state["needs_rename"] += 1
            
            ai_tags = cache.get("ai", {})
            target_name = ai_tags.get("filename", f.stem)
            slideshow_name = f"{target_name}_slideshow.mp4"
            if not (f.parent / ".aicli_cache" / "slideshows" / slideshow_name).exists():
                state["needs_compress"] += 1

        return state

    @staticmethod
    def print_preflight_dashboard(state: dict, target_dir: Path):
        stats = Table(title="Pre-Flight Pipeline Stats", show_header=True, header_style="bold cyan", border_style="dim")
        stats.add_column("Phase", style="bold")
        stats.add_column("Task", style="dim")
        stats.add_column("Pending", justify="right", style="yellow")
        stats.add_column("Done", justify="right", style="green")
        
        total = len(state["raw_files"])
        needs_trans = len(state["needs_transcription"])
        needs_tag = state["needs_tagging"]
        needs_comp = state["needs_compress"]
        notes_exists = (target_dir / "Course_Merged_NoFluff.md").exists()

        stats.add_row("1. Transcribe", "Whisper → SRT + TXT", str(needs_trans), str(total - needs_trans))
        stats.add_row("2. Tag & Rename", "LM Studio → metadata + rename", str(needs_tag), str(total - needs_tag))
        stats.add_row("3. Compress", "NVENC → slideshow", str(needs_comp), str(total - needs_comp))
        stats.add_row("4. Merge", "FFmpeg concat", "—", "—")
        stats.add_row("5. Notes", "LM Studio → .md", "0" if notes_exists else "1", "1" if notes_exists else "0")

        console.print(stats)
        console.print()
