"""Read-only video metadata scanning via ffprobe."""
import json
import subprocess
from pathlib import Path
from typing import Dict, Any


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

    @staticmethod
    def has_subtitle_stream(video_path: Path) -> bool:
        """Check if the video container already has an embedded subtitle stream."""
        cmd = ["ffprobe", "-v", "quiet", "-print_format", "json",
               "-show_streams", "-select_streams", "s", str(video_path)]
        out = subprocess.run(cmd, capture_output=True, text=True)
        try:
            data = json.loads(out.stdout)
            return len(data.get("streams", [])) > 0
        except Exception:
            return False
