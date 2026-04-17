"""
GPU-accelerated video compression service using NVENC.

Converts videos to 240p with minimal file size at maximum speed
using the RTX 3090's hardware encoder.
"""

import subprocess
import shutil
from pathlib import Path
from typing import Optional


class CompressService:
    """Hardware-accelerated video compression via NVENC."""

    # Compression presets: (video_bitrate, audio_bitrate, audio_channels, nvenc_preset)
    PRESETS = {
        "ultralight": ("150k", "32k", 1, "p1"),   # Absolute minimum — lecture audio only
        "light":      ("250k", "48k", 1, "p1"),   # Good for lectures at 240p
        "balanced":   ("400k", "64k", 1, "p4"),   # Slightly better quality
    }

    @staticmethod
    def compress(
        video_path: Path,
        output_path: Optional[Path] = None,
        resolution: int = 240,
        preset: str = "light",
        overwrite: bool = False,
        crf: Optional[int] = None,
    ) -> Path:
        """
        Compress a video to the target resolution using NVENC.

        Args:
            video_path: Source video file.
            output_path: Destination path. Defaults to <name>_240p.mp4 in same dir.
            resolution: Target vertical resolution (default 240).
            preset: One of 'ultralight', 'light', 'balanced'.
            overwrite: If True, replace the original file.
            crf: Optional constant quality value (0-51). If set, overrides bitrate.

        Returns:
            Path to the compressed file.
        """
        if preset not in CompressService.PRESETS:
            raise ValueError(f"Unknown preset '{preset}'. Choose from: {list(CompressService.PRESETS.keys())}")

        v_bitrate, a_bitrate, a_channels, nvenc_preset = CompressService.PRESETS[preset]

        if output_path is None:
            if overwrite:
                output_path = video_path.with_suffix(f".tmp_compress.mp4")
            else:
                stem = video_path.stem
                output_path = video_path.parent / f"{stem}_{resolution}p.mp4"

        if output_path.exists() and not overwrite:
            raise FileExistsError(f"Output already exists: {output_path}. Use --overwrite.")

        # Build FFmpeg command
        cmd = [
            "ffmpeg", "-y", "-v", "quiet", "-stats",
            "-hwaccel", "cuda",                       # GPU-accelerated decoding
            "-i", str(video_path),
            # Video: NVENC H.264
            "-c:v", "h264_nvenc",
            "-preset", nvenc_preset,                   # p1 = fastest
            "-tune", "ll",                             # low-latency tuning for speed
            "-vf", f"scale=-2:{resolution}",           # Scale to target height, auto-width (even)
            "-pix_fmt", "yuv420p",
        ]

        if crf is not None:
            cmd += ["-cq", str(crf), "-b:v", "0"]     # Constant quality mode
        else:
            cmd += ["-b:v", v_bitrate]                 # Target bitrate mode

        cmd += [
            # Audio: AAC mono, aggressively compressed
            "-c:a", "aac",
            "-b:a", a_bitrate,
            "-ac", str(a_channels),
            "-ar", "22050",                            # 22kHz is fine for voice
            # Strip all metadata, subtitles, attachments — pure lean file
            "-map", "0:v:0",
            "-map", "0:a:0?",                          # '?' = don't fail if no audio
            "-movflags", "+faststart",                 # Web-optimized MP4
            str(output_path),
        ]

        result = subprocess.run(cmd, capture_output=True, text=True)

        if result.returncode != 0 or not output_path.exists():
            if output_path.exists():
                output_path.unlink()
            stderr_tail = (result.stderr or "")[-500:]
            raise RuntimeError(f"FFmpeg compression failed: {stderr_tail}")

        if overwrite:
            # Replace original with compressed version
            original_size = video_path.stat().st_size
            video_path.unlink()
            final_path = video_path.with_suffix(".mp4")
            shutil.move(str(output_path), str(final_path))
            return final_path

        return output_path

    @staticmethod
    def get_file_size_mb(path: Path) -> float:
        """Return file size in megabytes."""
        return path.stat().st_size / (1024 * 1024)
