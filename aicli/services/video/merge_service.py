import subprocess
import shutil
from pathlib import Path
from typing import List, Tuple
from datetime import timedelta
import re

class MergeService:
    """Service for merging videos, SRTs, and text files natively."""

    @staticmethod
    def get_video_duration(video_path: Path) -> float:
        """Extract exact duration in seconds using ffprobe."""
        cmd = [
            "ffprobe", "-v", "error", "-show_entries",
            "format=duration", "-of", "default=noprint_wrappers=1:nokey=1",
            str(video_path)
        ]
        result = subprocess.run(cmd, capture_output=True, text=True)
        try:
            return float(result.stdout.strip())
        except ValueError:
            raise ValueError(f"Could not determine duration for {video_path}")

    @staticmethod
    def merge_videos(video_paths: List[Path], output_path: Path) -> bool:
        """Native multi-file concat using FFmpeg. Assumes all files have identical properties."""
        if not video_paths:
            return False

        # Create a concat list file
        list_file = output_path.with_suffix('.txt')
        with open(list_file, 'w', encoding='utf-8') as f:
            for vp in video_paths:
                # ffmpeg requires single quotes and escaped inner quotes
                safe_path = str(vp.absolute()).replace("'", "'\\''")
                f.write(f"file '{safe_path}'\n")

        # Run concat demuxer
        cmd = [
            "ffmpeg", "-y", "-f", "concat", "-safe", "0",
            "-i", str(list_file),
            "-c", "copy",
            str(output_path)
        ]
        res = subprocess.run(cmd, capture_output=True, text=True)
        
        # Cleanup list file regardless of success
        if list_file.exists():
            list_file.unlink()

        return res.returncode == 0 and output_path.exists()

    @staticmethod
    def merge_txts(txt_paths: List[Path], output_path: Path) -> bool:
        """Simple text concatenation with proper spacing."""
        with open(output_path, 'w', encoding='utf-8') as out_f:
            for tp in txt_paths:
                if tp.exists():
                    out_f.write(f"--- Segment: {tp.stem} ---\n\n")
                    out_f.write(tp.read_text(encoding='utf-8', errors='replace'))
                    out_f.write("\n\n")
        return True

    @staticmethod
    def _parse_srt_timestamp(ts: str) -> timedelta:
        # 00:15:30,450
        h, m, s_ms = ts.split(':')
        s, ms = s_ms.split(',')
        return timedelta(hours=int(h), minutes=int(m), seconds=int(s), milliseconds=int(ms))

    @staticmethod
    def _format_srt_timestamp(td: timedelta) -> str:
        total_sec = int(td.total_seconds())
        ms = int(td.microseconds / 1000)
        h = total_sec // 3600
        m = (total_sec % 3600) // 60
        s = total_sec % 60
        return f"{h:02d}:{m:02d}:{s:02d},{ms:03d}"

    @staticmethod
    def merge_srts(video_srt_pairs: List[Tuple[Path, Path]], output_path: Path) -> bool:
        """Merge multiple SRTs by applying a cumulative duration offset from the accompanying videos."""
        current_offset = timedelta()
        global_index = 1

        with open(output_path, 'w', encoding='utf-8') as out_f:
            for video_path, srt_path in video_srt_pairs:
                if not srt_path.exists():
                    # Even if SRT is missing, we must accumulate the video's time so future SRTs don't desync
                    if video_path.exists():
                        dur_sec = MergeService.get_video_duration(video_path)
                        current_offset += timedelta(seconds=dur_sec)
                    continue

                content = srt_path.read_text(encoding='utf-8', errors='replace')
                
                # We need to parse each block
                # Format:
                # 1
                # 00:00:00,000 --> 00:00:05,000
                # text...
                # empty line
                
                blocks = content.strip().split('\n\n')
                for block in blocks:
                    lines = block.split('\n')
                    if len(lines) >= 3:
                        # Validate the timestamp line
                        ts_line = lines[1]
                        if ' --> ' in ts_line:
                            start_str, end_str = ts_line.split(' --> ')
                            try:
                                start_td = MergeService._parse_srt_timestamp(start_str.strip())
                                end_td = MergeService._parse_srt_timestamp(end_str.strip())
                                
                                new_start = MergeService._format_srt_timestamp(start_td + current_offset)
                                new_end = MergeService._format_srt_timestamp(end_td + current_offset)
                                
                                out_f.write(f"{global_index}\n")
                                out_f.write(f"{new_start} --> {new_end}\n")
                                for text_line in lines[2:]:
                                    out_f.write(f"{text_line}\n")
                                out_f.write("\n")
                                
                                global_index += 1
                            except Exception:
                                pass # Skip malformed block
                
                # Accumulate duration for the next SRT
                dur_sec = MergeService.get_video_duration(video_path)
                current_offset += timedelta(seconds=dur_sec)

        return True
