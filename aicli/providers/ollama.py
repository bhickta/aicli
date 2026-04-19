"""Implementation of ImageVisionProvider using Ollama via the OpenAI SDK."""

import base64
import io
import json
import time
from typing import Optional

from openai import OpenAI
from PIL import Image

from aicli.core.interfaces import ImageVisionProvider
from aicli.config import config

_MIME_MAP = {
    "jpg": "image/jpeg",
    "jpeg": "image/jpeg",
    "png": "image/png",
    "webp": "image/webp",
    "gif": "image/gif",
}

_REASONING_MODELS = {"qwen3", "deepseek-r1", "qwq", "phi4"}


class OllamaProvider(ImageVisionProvider):
    """Ollama provider for image vision and text completion."""

    def __init__(self) -> None:
        self.client = OpenAI(
            base_url=f"{config.ollama_base_url}/v1",
            api_key=config.ollama_api_key,
        )

    def _is_reasoning_model(self) -> bool:
        model_lower = config.model_name.lower()
        return any(rm in model_lower for rm in _REASONING_MODELS)

    def describe_image(
        self,
        image_path: str,
        prompt: str,
        system_prompt: Optional[str] = None,
        max_size: int = 1024,
        temperature: float = 0.1,
        max_tokens: int = 2000,
        max_retries: int = 3,
        retry_backoff_base: int = 2,
        allow_reasoning: bool = True,
        abort_event: Optional[object] = None,
    ) -> str:
        """Send a base64-encoded image + prompt to Ollama, return text."""
        base64_image = self._encode_image(image_path, max_dim=max_size)
        mime_type = self._get_mime_type(image_path)
        combined_text = f"{system_prompt}\n\n{prompt}" if system_prompt else prompt

        messages = [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": combined_text},
                    {
                        "type": "image_url",
                        "image_url": {"url": f"data:{mime_type};base64,{base64_image}"},
                    },
                ],
            }
        ]

        return self._complete_with_retry(
            messages, temperature, max_tokens, max_retries, retry_backoff_base
        )

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
        """Text-only completion via Ollama."""
        messages = []
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})
        messages.append({"role": "user", "content": prompt})
        return self._complete_with_retry(
            messages,
            temperature,
            max_tokens,
            max_retries,
            retry_backoff_base,
            allow_reasoning,
        )

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
                prefix = (
                    "IMPORTANT: Return ONLY valid JSON, no markdown fences, no explanation.\n\n"
                    if attempt > 0
                    else ""
                )
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
                if attempt < max_retries - 1:
                    time.sleep(retry_backoff_base**attempt)

        raise ValueError(
            f"Failed to parse JSON after {max_retries} attempts: {last_error}"
        )

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

    def _complete_with_retry(
        self,
        messages: list,
        temperature: float,
        max_tokens: int,
        max_retries: int,
        backoff_base: int,
        allow_reasoning: bool = True,
    ) -> str:
        """Execute completion with retry logic."""
        last_error = None
        for attempt in range(max_retries):
            try:
                extra_body = {}
                is_reasoning = self._is_reasoning_model()

                if is_reasoning and allow_reasoning:
                    extra_body["thinking"] = {"type": "enabled", "duration": "inf"}
                elif not allow_reasoning:
                    extra_body["thinking"] = {"type": "disabled"}

                kwargs = {
                    "model": config.model_name,
                    "messages": messages,
                    "temperature": temperature,
                    "max_tokens": max_tokens,
                }
                if extra_body:
                    kwargs["extra_body"] = extra_body

                response = self.client.chat.completions.create(**kwargs)
                content = response.choices[0].message.content
                if content:
                    return content.strip()
                reason = response.choices[0].finish_reason
                raise ValueError(
                    f"Ollama returned empty content (finish_reason: '{reason}')"
                )
            except Exception as e:
                last_error = e
                if attempt < max_retries - 1:
                    time.sleep(backoff_base**attempt)

        raise last_error

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
