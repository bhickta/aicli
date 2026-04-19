"""Service for generating compressed study notes from SRT transcripts via LLM provider."""

import re
import subprocess
from pathlib import Path
from typing import Optional

from langchain_core.prompts import PromptTemplate
from langchain_text_splitters import RecursiveCharacterTextSplitter

from aicli.providers import get_provider
from aicli.config import config as app_config
from aicli.services.video.prompts import NOTES_SYSTEM_PROMPT, CLEAN_SYSTEM_PROMPT


class NotesService:
    """Generates compressed study notes from SRT files via Ollama."""

    CHUNK_SIZE = app_config.notes_chunk_size

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
            "ffmpeg",
            "-y",
            "-v",
            "quiet",
            "-i",
            str(video_path),
            "-map",
            "0:s:0",
            "-c:s",
            "srt",
            str(tmp_srt),
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
            if re.match(
                r"\d{2}:\d{2}:\d{2},\d{3}\s*-->\s*\d{2}:\d{2}:\d{2},\d{3}", line
            ):
                continue
            lines.append(line)
        return " ".join(lines)

    @staticmethod
    def _call_llm(text_chunk: str, style: str = "bullet") -> str:
        """Send a text chunk to the configured LLM provider and return compressed notes."""
        sys_prompt = CLEAN_SYSTEM_PROMPT if style == "clean" else NOTES_SYSTEM_PROMPT

        user_template = PromptTemplate.from_template(
            "Condense the following transcript text into {style} notes:\n\n{text_chunk}"
        )
        rendered = user_template.format(style=style, text_chunk=text_chunk)

        provider = get_provider()
        return provider.complete_text(
            prompt=rendered,
            system_prompt=sys_prompt,
            temperature=app_config.notes_temperature,
            max_tokens=app_config.notes_max_tokens,
        )

    @staticmethod
    def generate_notes_from_text(text: str, style: str = "bullet") -> str:
        """Plain text → LangChain splitters → chunked LLM calls → merged notes."""
        if not text.strip():
            raise ValueError("Input text is empty.")

        # Use native LangChain splitter for robust context-aware chunking
        splitter = RecursiveCharacterTextSplitter(
            chunk_size=NotesService.CHUNK_SIZE,
            chunk_overlap=app_config.notes_chunk_overlap if hasattr(app_config, "notes_chunk_overlap") else 200,
            length_function=len,
        )
        chunks = splitter.split_text(text)

        all_notes = []
        for chunk in chunks:
            notes = NotesService._call_llm(chunk, style=style)
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
