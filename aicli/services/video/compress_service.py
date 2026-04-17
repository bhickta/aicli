"""
GPU-accelerated video compression service using NVENC.

Full GPU-resident pipeline: decode → scale → encode all happen on the GPU.
Frames never leave VRAM — zero CPU roundtrip.
"""

import subprocess
import shutil
from pathlib import Path
from typing import Optional


class CompressService:
    """Hardware-accelerated video compression via NVENC (full GPU pipeline)."""

    PRESETS = {
        "ultralight": ("150k", "32k", 1, "p1", 10),   # Absolute minimum — 10fps lecture
        "light":      ("250k", "48k", 1, "p1", 15),   # Good for lectures at 240p
        "balanced":   ("400k", "64k", 1, "p1", 24),   # Decent motion, still fast
        "slideshow":  ("500k", "48k", 1, "p4", "1/20"), # 1 frame/20s, standard AAC audio for MP4 compliance
    }

    @staticmethod
    def compress(
        video_path: Path,
        output_path: Optional[Path] = None,
        resolution: int = 240,
        preset: str = "light",
        overwrite: bool = False,
        crf: Optional[int] = None,
        fps: Optional[str] = None,
        fast_skip: bool = False,
        metadata_tags: dict = None,
        external_srt: Path = None,
        target_name: str = None,
    ) -> Path:
        """
        Compress a video to the target resolution using a full GPU-resident pipeline.

        The entire decode → scale → encode happens on the GPU. Frames never
        touch CPU RAM. Combined with FPS reduction, a 2-hour lecture finishes
        in ~10-20 seconds on an RTX 3090.

        Args:
            video_path: Source video file.
            output_path: Destination path. Defaults to <name>_240p.mp4 in same dir.
            resolution: Target vertical resolution (default 240). Use 0 for original.
            preset: One of 'ultralight', 'light', 'balanced', 'slideshow'.
            overwrite: If True, replace the original file.
            crf: Optional constant quality value (0-51). If set, overrides bitrate.
            fps: Override output framerate. None uses preset default.
            fast_skip: If True, tell decoder to ignore non-keyframes for ultra-fast skipping.
            metadata_tags: Dictionary of AI-generated tags to embed natively.
            external_srt: Path to an external SRT file to embed into the container.
            target_name: Output base name for the compressed file.

        Returns:
            Path to the compressed file.
        """
        if preset not in CompressService.PRESETS:
            raise ValueError(f"Unknown preset '{preset}'. Choose from: {list(CompressService.PRESETS.keys())}")

        v_bitrate, a_bitrate, a_channels, nvenc_preset, default_fps = CompressService.PRESETS[preset]
        target_fps = fps if fps is not None else default_fps

        if preset == "slideshow" and resolution == 240:
            resolution = 0  # Force original resolution for slideshows

        if output_path is None:
            if overwrite:
                output_path = video_path.with_suffix(".tmp_compress.mp4")
            else:
                stem = target_name if target_name else video_path.stem
                res_suffix = f"_{resolution}p" if resolution > 0 else "_slideshow"
                output_path = video_path.parent / f"{stem}{res_suffix}.mp4"

        cmd = [
            "ffmpeg", "-y", "-v", "quiet", "-stats",
            "-hwaccel", "cuda",
            "-hwaccel_output_format", "cuda",
        ]

        if fast_skip:
            cmd += ["-skip_frame", "nokey"]

        cmd += ["-i", str(video_path)]
        
        has_ext_srt = external_srt and external_srt.exists()
        if has_ext_srt:
            cmd += ["-i", str(external_srt)]

        if resolution > 0:
            cmd += ["-vf", f"scale_cuda=-2:{resolution}"]

        cmd += [
            "-c:v", "h264_nvenc",
            "-preset", nvenc_preset,
            "-tune", "ll",
            "-r", str(target_fps),
        ]

        if crf is not None:
            cmd += ["-cq", str(crf), "-b:v", "0"]
        else:
            cmd += ["-b:v", v_bitrate]

        # Audio settings
        if a_bitrate == "copy":
            cmd += ["-c:a", "copy"]                    # Pure original audio untouched
        else:
            cmd += [
                "-c:a", "aac",
                "-b:a", a_bitrate,
                "-ac", str(a_channels),
                "-ar", "22050",
            ]

        # Keep first video, first audio, ALL subtitles, and ALL metadata
        cmd += [
            "-map", "0:v:0",
            "-map", "0:a:0?",
        ]
        
        if has_ext_srt:
            cmd += ["-map", "1:s?", "-c:s", "mov_text"]
        else:
            cmd += ["-map", "0:s?", "-c:s", "mov_text"]
            
        cmd += [
            "-map_metadata", "0",           # Keep global title/metadata
            "-map_chapters", "0",           # Keep any chapters if present
            "-movflags", "+faststart",
        ]
        
        if metadata_tags:
            if metadata_tags.get("title"):       cmd += ["-metadata", f'title={metadata_tags["title"]}']
            if metadata_tags.get("subject"):     cmd += ["-metadata", f'genre={metadata_tags["subject"]}']
            if metadata_tags.get("description"): cmd += ["-metadata", f'comment={metadata_tags["description"]}']
            if metadata_tags.get("teacher"):     cmd += ["-metadata", f'artist={metadata_tags["teacher"]}']

        cmd += [str(output_path)]

        result = subprocess.run(cmd, capture_output=True, text=True)

        if result.returncode != 0 or not output_path.exists():
            if output_path.exists():
                output_path.unlink()
            stderr_tail = (result.stderr or "")[-500:]
            raise RuntimeError(f"FFmpeg compression failed: {stderr_tail}")

        if overwrite:
            video_path.unlink()
            final_path = video_path.with_suffix(".mp4")
            shutil.move(str(output_path), str(final_path))
            return final_path

        return output_path

    @staticmethod
    def get_file_size_mb(path: Path) -> float:
        """Return file size in megabytes."""
        return path.stat().st_size / (1024 * 1024)
