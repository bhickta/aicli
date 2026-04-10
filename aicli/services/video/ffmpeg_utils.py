import json
import subprocess
import shutil
import base64
from pathlib import Path
from typing import Dict, Any, Optional
import numpy as np


class FFprobeClient:
    """Handles read-only metadata scanning."""
    
    @staticmethod
    def get_duration(video_path: Path) -> float:
        cmd = ["ffprobe", "-v", "quiet", "-print_format", "json",
               "-show_format", str(video_path)]
        out = subprocess.run(cmd, capture_output=True, text=True)
        try:
            return float(json.loads(out.stdout)["format"].get("duration", 0))
        except Exception:
            return 0.0

    @staticmethod
    def read_existing_tags(video_path: Path) -> Dict[str, Any]:
        """Read all existing metadata tags from the video file via ffprobe."""
        cmd = ["ffprobe", "-v", "quiet", "-print_format", "json",
               "-show_format", "-show_streams", str(video_path)]
        out = subprocess.run(cmd, capture_output=True, text=True)
        try:
            data = json.loads(out.stdout)
            tags = data.get("format", {}).get("tags", {})
            for s in data.get("streams", []):
                tags.update(s.get("tags", {}))
            return tags
        except Exception:
            return {}


class FFmpegClient:
    """Handles destructive or modifying video operations."""
    
    @staticmethod
    def generate_thumbnail(video_path: Path, output_path: Path, duration: float) -> bool:
        """Extract a single frame from the video to serve as a thumbnail."""
        target_sec = duration * 0.15 if duration > 0 else 5.0
        
        cmd = [
            "ffmpeg", "-y", "-v", "quiet",
            "-ss", str(target_sec),
            "-i", str(video_path),
            "-frames:v", "1",
            "-q:v", "2",
            str(output_path)
        ]
        
        result = subprocess.run(cmd)
        return result.returncode == 0 and output_path.exists()

    @staticmethod
    def stream_audio_clip(video_path: Path, start_sec: float, duration_sec: float) -> Optional[np.ndarray]:
        """Stream audio clip directly into memory as float32 numpy array."""
        cmd = [
            "ffmpeg", "-v", "quiet",
            "-ss", str(start_sec),
            "-i", str(video_path),
            "-t", str(duration_sec),
            "-vn", "-ac", "1", "-ar", "16000",
            "-f", "f32le", "pipe:1"
        ]
        result = subprocess.run(cmd, capture_output=True)
        if not result.stdout:
            return None
        return np.frombuffer(result.stdout, dtype=np.float32)

    @staticmethod
    def write_tags(video_path: Path, tags: Dict[str, Any], clear_first: bool = False, original_tags: Optional[Dict[str, Any]] = None) -> bool:
        """Write metadata tags using ffmpeg stream copy (no re-encode). Embeds original tags as b64 if provided."""
        tmp = video_path.with_suffix(".tmp_tagged.mp4")

        meta_args = []
        if clear_first:
            meta_args += ["-map_metadata", "-1"]
        else:
            meta_args += ["-map_metadata", "0"]

        for k, v in tags.items():
            if v and k.lower() != "aicli_backup":
                val = ", ".join(v) if isinstance(v, list) else str(v)
                if k == "language_track":
                    meta_args += ["-metadata:s:a", f"language={val}"]
                else:
                    meta_args += ["-metadata", f"{k}={val}"]
                
        if original_tags:
            b64_backup = base64.b64encode(json.dumps(original_tags).encode("utf-8")).decode("utf-8")
            meta_args += ["-metadata", f"aicli_backup={b64_backup}"]

        cmd = ["ffmpeg", "-y", "-v", "quiet",
               "-i", str(video_path), "-c", "copy",
               *meta_args, str(tmp)]

        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0 or not tmp.exists():
            if tmp.exists(): tmp.unlink()
            raise RuntimeError(f"FFmpeg tag write error: {result.stderr[-200:]}")
            
        shutil.move(str(tmp), str(video_path))
        return True
