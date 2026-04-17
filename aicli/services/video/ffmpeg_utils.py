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


class FFmpegClient:
    """Handles destructive or modifying video operations."""

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
    def generate_text_thumbnail(title: str, output_path: Path) -> bool:
        """Create a text-based image thumbnail for video cover art using Pillow."""
        try:
            from PIL import Image, ImageDraw, ImageFont
        except ImportError:
            return False
            
        import textwrap
        import os
        
        img = Image.new('RGB', (1280, 720), color=(44, 62, 80)) # Slate Blue
        d = ImageDraw.Draw(img)
        
        font = ImageFont.load_default()
        paths = [
            "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
            "/usr/share/fonts/opentype/urw-base35/NimbusSans-Bold.otf",
            "/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf",
            "/usr/share/fonts/gnu-free/FreeSansBold.ttf",
            "/usr/share/fonts/truetype/ubuntu/Ubuntu-B.ttf"
        ]
        for p in paths:
            if os.path.exists(p):
                font = ImageFont.truetype(p, 72)
                break

        wrapper = textwrap.TextWrapper(width=30)
        lines = wrapper.wrap(text=title)
        
        line_height = 80 
        total_height = len(lines) * line_height
        current_y = (720 - total_height) / 2
        
        for line in lines:
            try:
                left, top, right, bottom = d.textbbox((0,0), line, font=font)
                width = right - left
            except AttributeError:
                width = d.textlength(line, font=font) if hasattr(d, 'textlength') else 500
            
            x = (1280 - width) / 2
            d.text((x, current_y), line, fill=(236, 240, 241), font=font)
            current_y += line_height
            
        img.save(str(output_path), quality=90)
        return output_path.exists()

    @staticmethod
    def write_tags(
        video_path: Path, 
        tags: Dict[str, Any], 
        clear_first: bool = False, 
        original_tags: Optional[Dict[str, Any]] = None,
        srt_path: Optional[Path] = None,
        cover_path: Optional[Path] = None
    ) -> bool:
        """Write metadata tags using ffmpeg stream copy (no re-encode). Embeds original tags as b64 if provided. Embeds SRT and Cover Art if provided."""
        tmp = video_path.with_suffix(".tmp_tagged.mp4" if video_path.suffix.lower() == ".mp4" else ".tmp_tagged.mkv")

        cmd = ["ffmpeg", "-y", "-v", "quiet", "-i", str(video_path)]
        
        if srt_path and srt_path.exists():
            cmd += ["-i", str(srt_path)]
            
        if cover_path and cover_path.exists() and video_path.suffix.lower() == ".mp4":
            cmd += ["-i", str(cover_path)]
            
        cmd += ["-map", "0"]
        input_idx = 1
        
        if srt_path and srt_path.exists():
            cmd += ["-map", f"{input_idx}:0"]
            input_idx += 1
            
        if cover_path and cover_path.exists() and video_path.suffix.lower() == ".mp4":
            cmd += ["-map", f"{input_idx}:0"]
            
        cmd += ["-c", "copy"]
        
        if srt_path and srt_path.exists():
            if video_path.suffix.lower() == ".mp4":
                cmd += ["-c:s", "mov_text"]
            else:
                cmd += ["-c:s", "srt"]

        if cover_path and cover_path.exists():
            if video_path.suffix.lower() == ".mp4":
                cmd += ["-disposition:v:1", "attached_pic"]
            else:
                cmd += ["-attach", str(cover_path), "-metadata:s:t", "mimetype=image/jpeg"]

        meta_args = []

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

        cmd += [*meta_args, str(tmp)]

        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0 or not tmp.exists():
            if tmp.exists(): tmp.unlink()
            raise RuntimeError(f"FFmpeg tag write error: {result.stderr[-200:]}")
            
        shutil.move(str(tmp), str(video_path))
        return True
