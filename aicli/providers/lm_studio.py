"""Implementation of ImageVisionProvider using LM Studio via the OpenAI SDK."""
import base64
import io
import json
import re
import time
from typing import Optional

from openai import OpenAI
from PIL import Image

from aicli.core.interfaces import ImageVisionProvider
from aicli.config import config

# Regex to strip leaked reasoning/thought blocks from model output
_THOUGHT_PATTERNS = [
    re.compile(r'<\|channel\|>thought.*?<\|channel\|>', re.DOTALL),
    re.compile(r'<\|channel\|>thought.*', re.DOTALL),
    re.compile(r'<think>.*?</think>', re.DOTALL),
    re.compile(r'<thought>.*?</thought>', re.DOTALL),
]

_MIME_MAP = {
    "jpg": "image/jpeg", "jpeg": "image/jpeg",
    "png": "image/png", "webp": "image/webp", "gif": "image/gif",
}


class LMStudioProvider(ImageVisionProvider):
    """LM Studio provider for image vision and text completion."""

    def __init__(self) -> None:
        self.client = OpenAI(
            base_url=config.lm_studio_base_url,
            api_key=config.lm_studio_api_key,
        )

    def describe_image(
        self,
        image_path: str,
        prompt: str,
        system_prompt: Optional[str] = None,
        max_size: int = 512,
        temperature: float = 0.2,
        max_tokens: Optional[int] = None,
        max_retries: int = 1,
        retry_backoff_base: int = 2,
        allow_reasoning: bool = True,
    ) -> str:
        """Send a base64-encoded image + prompt to LM Studio, return text."""
        base64_image = self._encode_image(image_path, max_dim=max_size)
        mime_type = self._get_mime_type(image_path)
        combined_text = f"{system_prompt}\n\n{prompt}" if system_prompt else prompt

        messages = [{"role": "user", "content": [
            {"type": "text", "text": combined_text},
            {"type": "image_url", "image_url": {"url": f"data:{mime_type};base64,{base64_image}"}},
        ]}]

        kwargs = self._build_create_kwargs(messages, temperature, max_tokens)
        return self._retry_completion(kwargs, max_retries, retry_backoff_base)

    def complete_text(
        self,
        prompt: str,
        system_prompt: Optional[str] = None,
        temperature: float = 0.1,
        max_tokens: int = 2000,
        max_retries: int = 1,
        retry_backoff_base: int = 2,
        allow_reasoning: bool = True,
    ) -> str:
        """Text-only completion via LM Studio."""
        messages = self._build_text_messages(prompt, system_prompt)
        kwargs = self._build_create_kwargs(messages, temperature, max_tokens)
        return self._retry_text_completion(kwargs, allow_reasoning, max_retries, retry_backoff_base)

    def complete_text_json(
        self,
        prompt: str,
        system_prompt: Optional[str] = None,
        temperature: float = 0.1,
        max_tokens: int = 2000,
        max_retries: int = 3,
        retry_backoff_base: int = 2,
        allow_reasoning: bool = True,
    ) -> dict:
        """Text completion that parses the response as JSON."""
        last_error = None
        for attempt in range(max_retries):
            try:
                prefix = "IMPORTANT: Return ONLY valid JSON, no markdown fences, no explanation.\n\n" if attempt > 0 else ""
                raw = self.complete_text(
                    prompt=prefix + prompt,
                    system_prompt=system_prompt,
                    temperature=temperature,
                    max_tokens=max_tokens,
                    max_retries=1,
                    allow_reasoning=allow_reasoning,
                )
                return self._parse_json_response(raw)
            except (json.JSONDecodeError, ValueError) as e:
                last_error = e
                self._backoff(attempt, max_retries, retry_backoff_base)

        raise ValueError(f"Failed to parse JSON after {max_retries} attempts: {last_error}")

    # ── Private: Image Encoding ─────────────────────────────────────

    @staticmethod
    def _encode_image(image_path: str, max_dim: int = 512) -> str:
        """Read, resize, and base64-encode an image."""
        with Image.open(image_path) as img:
            if img.mode in ("RGBA", "P"):
                img = img.convert("RGB")
            if max(img.size) > max_dim:
                img.thumbnail((max_dim, max_dim), Image.Resampling.LANCZOS)
            buffer = io.BytesIO()
            img.save(buffer, format="JPEG", quality=85)
            return base64.b64encode(buffer.getvalue()).decode("utf-8")

    @staticmethod
    def _get_mime_type(image_path: str) -> str:
        ext = image_path.lower().rsplit(".", 1)[-1]
        return _MIME_MAP.get(ext, "image/jpeg")

    # ── Private: Message Building ───────────────────────────────────

    @staticmethod
    def _build_text_messages(prompt: str, system_prompt: Optional[str]) -> list:
        messages = []
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})
        messages.append({"role": "user", "content": prompt})
        return messages

    @staticmethod
    def _build_create_kwargs(messages: list, temperature: float, max_tokens: Optional[int]) -> dict:
        kwargs = {"model": config.model_name, "messages": messages, "temperature": temperature}
        if max_tokens:
            kwargs["max_tokens"] = max_tokens
        return kwargs

    # ── Private: Retry Logic ────────────────────────────────────────

    def _retry_completion(self, kwargs: dict, max_retries: int, backoff_base: int) -> str:
        """Simple retry loop for image completions."""
        last_error = None
        for attempt in range(max_retries):
            try:
                return self._call_and_extract(kwargs)
            except Exception as e:
                last_error = e
                self._backoff(attempt, max_retries, backoff_base)
        raise last_error

    def _retry_text_completion(
        self, kwargs: dict, allow_reasoning: bool, max_retries: int, backoff_base: int
    ) -> str:
        """Retry loop for text completions with reasoning control and 400-error fallback."""
        last_error = None
        for attempt in range(max_retries):
            try:
                return self._call_with_reasoning(kwargs, allow_reasoning)
            except Exception as e:
                if self._is_reasoning_rejection(e):
                    return self._fallback_without_reasoning(kwargs)
                last_error = e
                self._backoff(attempt, max_retries, backoff_base)
        raise last_error

    def _call_and_extract(self, kwargs: dict) -> str:
        """Execute completion and extract content, raising on empty."""
        response = self.client.chat.completions.create(**kwargs)
        content = response.choices[0].message.content
        if content:
            return content.strip()
        reason = response.choices[0].finish_reason
        raise ValueError(f"LM Studio returned empty content (finish_reason: '{reason}')")

    def _call_with_reasoning(self, kwargs: dict, allow_reasoning: bool) -> str:
        """Execute completion with optional reasoning parameter."""
        enhanced = {**kwargs, "extra_body": {"reasoning": allow_reasoning}}
        content = self._call_and_extract(enhanced)
        return self._strip_thought_blocks(content)

    def _fallback_without_reasoning(self, kwargs: dict) -> str:
        """Fallback call without extra_body when the server rejects the reasoning param."""
        try:
            return self._call_and_extract(kwargs)
        except Exception:
            raise

    # ── Private: Response Cleaning ──────────────────────────────────

    @staticmethod
    def _strip_thought_blocks(content: str) -> str:
        """Remove leaked reasoning/thought blocks from output."""
        for pattern in _THOUGHT_PATTERNS:
            content = pattern.sub("", content)
        return content.strip()

    @staticmethod
    def _parse_json_response(raw: str) -> dict:
        """Strip markdown fences and parse JSON."""
        cleaned = raw.strip()
        if cleaned.startswith("```"):
            lines = cleaned.split("\n")
            if lines[0].startswith("```"):
                lines = lines[1:]
            if lines and lines[-1].strip() == "```":
                lines = lines[:-1]
            cleaned = "\n".join(lines)
        return json.loads(cleaned)

    @staticmethod
    def _is_reasoning_rejection(error: Exception) -> bool:
        return "400" in str(error)

    @staticmethod
    def _backoff(attempt: int, max_retries: int, base: int) -> None:
        if attempt < max_retries - 1:
            time.sleep(base ** attempt)
