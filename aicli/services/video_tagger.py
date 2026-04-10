import os
import json
import subprocess
import shutil
import urllib.request
import urllib.error
from pathlib import Path
from typing import Dict, Any, List, Optional
import numpy as np

from aicli.config import config


class VideoTaggerService:
    """Service to handle video transcription, AI tagging, and metadata editing."""

    VIDEO_EXTENSIONS = {".mp4", ".mkv", ".mov", ".avi", ".webm", ".m4v", ".ts", ".mts"}

    @staticmethod
    def sidecar_path(video_path: Path) -> Path:
        return video_path.with_suffix(".sidecar.json")

    @staticmethod
    def load_cache(video_path: Path) -> Dict[str, Any]:
        sp = VideoTaggerService.sidecar_path(video_path)
        if sp.exists():
            try:
                return json.loads(sp.read_text(encoding="utf-8"))
            except:
                pass
        return {}

    @staticmethod
    def save_cache(video_path: Path, data: Dict[str, Any]) -> None:
        sp = VideoTaggerService.sidecar_path(video_path)
        sp.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")

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
        except:
            return {}

    @staticmethod
    def backup_original_tags(video_path: Path, cache: Dict[str, Any]) -> tuple[Dict[str, Any], bool]:
        """Save original tags into sidecar once. Returns updated cache and boolean if written just now."""
        if "original_tags" not in cache:
            tags = VideoTaggerService.read_existing_tags(video_path)
            cache["original_tags"] = tags
            return cache, True
        return cache, False

    @staticmethod
    def restore_original_tags(video_path: Path) -> bool:
        """Restore the video's original tags from the sidecar backup."""
        sp = VideoTaggerService.sidecar_path(video_path)
        if not sp.exists():
            raise FileNotFoundError(f"No sidecar found for {video_path.name}")
        
        cache = json.loads(sp.read_text(encoding="utf-8"))
        original = cache.get("original_tags")
        if not original:
            raise ValueError("No original_tags backup found in sidecar.")
            
        return VideoTaggerService.write_tags(video_path, original, clear_first=True)

    @staticmethod
    def get_duration(path: Path) -> float:
        cmd = ["ffprobe", "-v", "quiet", "-print_format", "json",
               "-show_format", str(path)]
        out = subprocess.run(cmd, capture_output=True, text=True)
        try:
            return float(json.loads(out.stdout)["format"].get("duration", 0))
        except:
            return 0.0

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
    def compute_clip_times(duration: float, clip_every: int, clip_len: int) -> List[float]:
        if duration < clip_len:
            return [0]
        margin = duration * 0.05
        times, t = [], margin
        while t <= duration - margin - clip_len:
            times.append(t)
            t += clip_every
        return times

    @staticmethod
    def load_whisper(model_size: str):
        try:
            from faster_whisper import WhisperModel
        except ImportError:
            raise ImportError("faster-whisper is not installed. Run: pip install faster-whisper numpy")
        return WhisperModel(model_size, device="cuda", compute_type="float16")

    @staticmethod
    def transcribe_video(video_path: Path, whisper_model, clip_every: int, clip_len: int, callback=None) -> List[Dict[str, Any]]:
        duration = VideoTaggerService.get_duration(video_path)
        times = VideoTaggerService.compute_clip_times(duration, clip_every, clip_len)
        results = []
        
        for i, t in enumerate(times):
            audio = VideoTaggerService.stream_audio_clip(video_path, t, clip_len)
            if audio is None or len(audio) < 1600:
                continue
            segments, _ = whisper_model.transcribe(
                audio, beam_size=5, language=None, vad_filter=True
            )
            text = " ".join(s.text.strip() for s in segments).strip()
            if text:
                results.append({"start_sec": round(t, 1), "text": text})
                if callback:
                    callback(len(times), i+1, t, text)
                    
        return results

    @staticmethod
    def ask_lmstudio(clips: List[Dict[str, Any]], path_hint: str) -> Optional[Dict[str, Any]]:
        transcript = "\n".join(
            f"[{int(c['start_sec']//60)}m{int(c['start_sec']%60)}s] {c['text']}"
            for c in clips
        )

        system = """You are a lecture metadata extractor.
Return ONLY valid JSON — no markdown, no extra text:
{
  "title": "concise descriptive lecture title",
  "filename": "snake_case_max_60_chars",
  "subject": "academic subject",
  "topics": ["topic1", "topic2", "topic3"],
  "description": "2-3 sentence summary",
  "language": "detected language"
}"""

        payload = json.dumps({
            "model": config.model_name,
            "messages": [
                {"role": "system", "content": system},
                {"role": "user",   "content": f"Folder: {path_hint}\n\n{transcript[:4000]}"}
            ],
            "temperature": 0.2,
            "max_tokens": 400,
        }).encode()

        endpoint = f"{config.lm_studio_base_url}/chat/completions"
        req = urllib.request.Request(
            endpoint,
            data=payload,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {config.lm_studio_api_key}"
            },
            method="POST"
        )
        try:
            with urllib.request.urlopen(req, timeout=60) as resp:
                text = json.loads(resp.read())["choices"][0]["message"]["content"].strip()
                text = text.replace("```json", "").replace("```", "").strip()
                return json.loads(text)
        except urllib.error.URLError as e:
            raise ConnectionError(f"LM Studio Error: {e}")
        except Exception as e:
            raise ValueError(f"Failed to parse response: {e}")

    @staticmethod
    def write_tags(video_path: Path, tags: Dict[str, Any], clear_first: bool = False) -> bool:
        """Write metadata tags using ffmpeg stream copy (no re-encode)."""
        tmp = video_path.with_suffix(".tmp_tagged.mp4")

        meta_args = []
        if clear_first:
            meta_args += ["-map_metadata", "-1"]
        else:
            meta_args += ["-map_metadata", "0"]

        for k, v in tags.items():
            if v:
                val = ", ".join(v) if isinstance(v, list) else str(v)
                meta_args += ["-metadata", f"{k}={val}"]

        cmd = ["ffmpeg", "-y", "-v", "quiet",
               "-i", str(video_path), "-c", "copy",
               *meta_args, str(tmp)]

        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0 or not tmp.exists():
            if tmp.exists(): tmp.unlink()
            raise RuntimeError(f"FFmpeg tag write error: {result.stderr[-200:]}")
            
        shutil.move(str(tmp), str(video_path))
        return True
