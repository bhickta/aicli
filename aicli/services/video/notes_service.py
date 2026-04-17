"""Service for generating compressed study notes from SRT transcripts via LM Studio."""
import json
import re
import subprocess
import urllib.request
import urllib.error
from pathlib import Path
from typing import Optional

from aicli.config import config


NOTES_SYSTEM_PROMPT = """Your Role: You are an Expert AI Keyword Extractor and Cognitive Compressor.
Your Goal: Transform the provided source text into ultra-dense, exam-ready bullet notes. The output must be optimized for rapid information recall and memory retention.
Mandatory Core Principles:
- **Focus**: Emphasize examples, academic terms, and jargon.
- **Extreme Conciseness**: No verbose language, strictly no grammar, eliminate obvious explanations. Be space and word efficient but cover everything.
- **Atomic Units**: Condense all information for a single concept into one line.

Notes MUST be written entirely in English.

Strict Source Integrity & Zero Information Loss:
- Extract information only from the provided source text. Do not infer, add external knowledge, or fill gaps.
- CRITICAL RULE: Ensure there is absolutely no loss of any information, examples, facts, dimensions, or concepts. Every detail from the source must be preserved in the compressed output.
- Correct only obvious, unambiguous typos.

Logical Grouping (DRY Principle):
- Cluster related ideas under a single main bullet point.
- Use indentation to create a clear conceptual hierarchy.
- Consolidate repeated concepts to avoid redundancy.

Prioritization of Key Data:
- Retain all specific data: proper nouns (names of people, places, policies, scientific terms), key examples, and high-impact statistics (e.g., percentages, years, ranks, quantities).

Mandatory Output Format & Style (Ultra-Compact):
- Use hyphen (-) for top-level bullets and a single tab (\\t) for indentation.
- Bold the primary term. All related information (mechanisms, effects, examples) MUST be on the same line.
- NO sub-bullets. NO new lines for a single concept.
- NO headings, horizontal rules (---), or blank lines. The output must be a single, continuous block of dense notes.
- **Example:**
\t- **Allelopathy**: Mechanism Some roots release **phytotoxins**, inhibit growth, or stop seed germination.

Instruction:
Process the following text according to all rules specified above."""

CLEAN_SYSTEM_PROMPT = """Your Role: You are an Expert Transcript Editor and Content Cleaner.
Your Goal: Clean and format the provided raw transcript without losing ANY informational content.

Mandatory Core Principles:
- **Strict No-Information-Loss Policy**: You must preserve every single fact, concept, example, explanation, and nuance from the source text. Do NOT summarize or condense the core educational content.
- **Fluff Removal ONLY**: You may ONLY remove genuine fluff: sponsor ads, "subscribe", irrelevant tangents, "how are you guys doing" chatter, verbal tics, and completely off-topic banter.
- **Readable Formatting**: Format the cleaned text into well-structured, readable paragraphs with clear headings if topics shift. Do NOT forcefully compress into ultra-dense bullets. Maintain the conversational/educational flow.
- **Accuracy**: Fix obvious auto-captioning typos, but do not alter the speaker's original meaning.

Output the perfectly cleaned transcript in clear, highly readable English prose."""


class NotesService:
    """Generates compressed study notes from SRT files via LM Studio."""

    # Maximum characters to send in a single LM Studio request
    CHUNK_SIZE = 12000

    @staticmethod
    def has_subtitle_stream(video_path: Path) -> bool:
        """Check if a video file has an embedded subtitle stream."""
        cmd = [
            "ffprobe", "-v", "quiet", "-print_format", "json",
            "-show_streams", "-select_streams", "s",
            str(video_path)
        ]
        result = subprocess.run(cmd, capture_output=True, text=True)
        try:
            data = json.loads(result.stdout)
            return len(data.get("streams", [])) > 0
        except Exception:
            return False

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
                {"role": "user", "content": text_chunk},
            ],
            "temperature": 0.15,
            "max_tokens": 4096,
        }).encode()

        endpoint = f"{config.lm_studio_base_url}/chat/completions"
        req = urllib.request.Request(
            endpoint,
            data=payload,
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer {config.lm_studio_api_key}",
            },
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=120) as resp:
                body = json.loads(resp.read())
                return body["choices"][0]["message"]["content"].strip()
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
