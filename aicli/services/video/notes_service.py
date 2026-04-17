"""Service for generating compressed study notes from SRT transcripts via LM Studio."""
import json
import re
import subprocess
import urllib.request
import urllib.error
from pathlib import Path
from typing import Optional

from aicli.config import config
from aicli.services.video.prompts import NOTES_SYSTEM_PROMPT, CLEAN_SYSTEM_PROMPT


class NotesService:
    """Generates compressed study notes from SRT files via LM Studio."""

    # Maximum characters to send in a single LM Studio request
    CHUNK_SIZE = 12000

    @staticmethod
    def has_subtitle_stream(video_path: Path) -> bool:
        """Check if a video file has an embedded subtitle stream."""
        from aicli.services.video.ffprobe import FFprobeClient
        return FFprobeClient.has_subtitle_stream(video_path)

    @staticmethod
    def extract_srt_from_video(video_path: Path) -> Optional[Path]:
        """Extract the first subtitle stream from a video container to a temp .srt file."""
        tmp_srt = video_path.with_suffix(".tmp_notes.srt")
        cmd = [
            "ffmpeg", "-y", "-v", "quiet",
            "-i", str(video_path),
            "-map", "0:s:0",  # First subtitle stream
            "-c:s", "srt",
            str(tmp_srt)
        ]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode == 0 and tmp_srt.exists() and tmp_srt.stat().st_size > 0:
            return tmp_srt
        if tmp_srt.exists():
            tmp_srt.unlink()
        return None

    @staticmethod
    def srt_to_text(srt_path: Path) -> str:
        """Strip SRT formatting (indices + timestamps) and return clean plain text."""
        content = srt_path.read_text(encoding="utf-8", errors="replace")
        lines = []
        for line in content.splitlines():
            line = line.strip()
            if not line:
                continue
            if re.match(r"^\d+$", line):
                continue
            if re.match(r"\d{2}:\d{2}:\d{2},\d{3}\s*-->\s*\d{2}:\d{2}:\d{2},\d{3}", line):
                continue
            lines.append(line)
        return " ".join(lines)

    @staticmethod
    def _call_lmstudio(text_chunk: str, style: str = "bullet") -> str:
        """Send a text chunk to LM Studio and return the compressed notes."""
        sys_prompt = CLEAN_SYSTEM_PROMPT if style == "clean" else NOTES_SYSTEM_PROMPT

        payload = json.dumps({
            "model": config.model_name,
            "messages": [
                {"role": "system", "content": sys_prompt},
                {"role": "user", "content": f"Condense the following transcript text into {style} notes:\n\n{text_chunk}"}
            ],
            "temperature": 0.2,
            "max_tokens": 2048,
            "stream": False
        }).encode('utf-8')

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
            with urllib.request.urlopen(req, timeout=120) as resp:
                data = json.loads(resp.read())
                if "choices" in data and len(data["choices"]) > 0:
                    text = data["choices"][0]["message"].get("content", "").strip()
                else:
                    text = ""
                return text
        except urllib.error.URLError as e:
            raise ConnectionError(f"LM Studio Error: {e}")
        except Exception as e:
            raise ValueError(f"Failed to parse LM Studio response: {e}")

    @staticmethod
    def generate_notes_from_text(text: str, style: str = "bullet") -> str:
        """Plain text → chunked LM Studio calls → merged notes."""
        if not text.strip():
            raise ValueError("Input text is empty.")

        # Split into chunks to avoid blowing up context window
        chunks = []
        words = text.split()
        current = []
        current_len = 0
        for word in words:
            current.append(word)
            current_len += len(word) + 1
            if current_len >= NotesService.CHUNK_SIZE:
                chunks.append(" ".join(current))
                current = []
                current_len = 0
        if current:
            chunks.append(" ".join(current))

        all_notes = []
        for chunk in chunks:
            notes = NotesService._call_lmstudio(chunk, style=style)
            if notes:
                all_notes.append(notes)

        return "\n\n".join(all_notes)

    @staticmethod
    def generate_notes(srt_path: Path, style: str = "bullet") -> str:
        """Full pipeline: SRT file → plain text → notes."""
        text = NotesService.srt_to_text(srt_path)
        return NotesService.generate_notes_from_text(text, style=style)

    @staticmethod
    def save_notes(video_path: Path, notes_content: str) -> Path:
        """Save notes as .md file next to the video."""
        md_path = video_path.with_suffix(".md")
        md_path.write_text(notes_content, encoding="utf-8")
        return md_path
