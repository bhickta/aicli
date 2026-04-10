import json
import urllib.request
import urllib.error
from typing import Dict, Any, List, Optional

from aicli.config import config


class VideoTaggerService:
    """High-level Orchestrator for video metadata generation via LM Studio."""
    
    VIDEO_EXTENSIONS = {".mp4", ".mkv", ".mov", ".avi", ".webm", ".m4v", ".ts", ".mts"}

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
  "filename": "Title Case with Spaces max 60 chars",
  "subject": "academic subject",
  "topics": ["topic1", "topic2", "topic3"],
  "description": "2-3 sentence summary",
  "teacher": "name of the teacher/professor",
  "coaching": "name of the coaching institute/channel",
  "language": "3-letter ISO-639-2 lowercase code (e.g. hin, eng)"
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
