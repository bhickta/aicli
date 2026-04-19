"""Base class for Langchain-powered AI Providers."""

import base64
import io
import json
import time
import logging
from typing import Optional

from PIL import Image

from aicli.core.interfaces import ImageVisionProvider
from aicli.config import config as app_config
from langchain_core.messages import HumanMessage, SystemMessage
from langchain_core.language_models.chat_models import BaseChatModel

_MIME_MAP = {
    "jpg": "image/jpeg",
    "jpeg": "image/jpeg",
    "png": "image/png",
    "webp": "image/webp",
    "gif": "image/gif",
}

logger = logging.getLogger(__name__)

class LangChainProvider(ImageVisionProvider):
    """Generic provider mapped to any LangChain Chat Model interface."""

    def __init__(self, llm: BaseChatModel) -> None:
        self.llm = llm

    def _log_llm_call(self, res, latency_ms: float):
        """Helper to log token counts and latency from AIMessage response wrapper."""
        try:
            tokens = res.response_metadata.get("token_usage", {})
            p_tokens = tokens.get("prompt_tokens", 0)
            c_tokens = tokens.get("completion_tokens", 0)
            model_name = res.response_metadata.get("model_name", "unknown_model")
            logger.info("LLM call | model=%s | tokens=%d+%d | latency=%.0fms", model_name, p_tokens, c_tokens, latency_ms)
        except Exception:
            pass

    def describe_image(
        self,
        image_path: str,
        prompt: str,
        system_prompt: Optional[str] = None,
        max_size: int = 1024,
        temperature: float = None,
        max_tokens: int = None,
        max_retries: int = None,
        retry_backoff_base: int = None,
        allow_reasoning: bool = True,
        abort_event: Optional[object] = None,
    ) -> str:
        temperature = temperature if temperature is not None else app_config.llm_default_temperature
        max_tokens = max_tokens if max_tokens is not None else app_config.llm_default_max_tokens
        max_retries = max_retries if max_retries is not None else app_config.llm_default_max_retries
        retry_backoff_base = retry_backoff_base if retry_backoff_base is not None else app_config.llm_retry_backoff_base
        base64_image = self._encode_image(image_path, max_dim=max_size)
        mime_type = self._get_mime_type(image_path)

        messages = []
        if system_prompt:
            messages.append(SystemMessage(content=system_prompt))
            
        messages.append(HumanMessage(content=[
            {"type": "text", "text": prompt},
            {
                "type": "image_url",
                "image_url": {"url": f"data:{mime_type};base64,{base64_image}"}
            }
        ]))
        
        kwargs = {"temperature": temperature, "max_tokens": max_tokens}
        
        try:
            start_t = time.perf_counter()
            res = self.llm.invoke(messages, **kwargs)
            latency_ms = (time.perf_counter() - start_t) * 1000
            self._log_llm_call(res, latency_ms)
            return res.content
        except Exception as e:
            for attempt in range(max_retries - 1):
                try:
                    time.sleep(retry_backoff_base ** attempt)
                    res = self.llm.invoke(messages, **kwargs)
                    return res.content
                except Exception:
                    pass
            raise e

    def complete_text(
        self,
        prompt: str,
        system_prompt: Optional[str] = None,
        temperature: float = None,
        max_tokens: int = None,
        max_retries: int = None,
        retry_backoff_base: int = None,
        allow_reasoning: bool = True,
    ) -> str:
        temperature = temperature if temperature is not None else app_config.llm_default_temperature
        max_tokens = max_tokens if max_tokens is not None else app_config.llm_default_max_tokens
        max_retries = max_retries if max_retries is not None else app_config.llm_default_max_retries
        retry_backoff_base = retry_backoff_base if retry_backoff_base is not None else app_config.llm_retry_backoff_base
        messages = []
        if system_prompt:
            messages.append(SystemMessage(content=system_prompt))
        messages.append(HumanMessage(content=prompt))

        kwargs = {"temperature": temperature, "max_tokens": max_tokens}
        
        try:
            start_t = time.perf_counter()
            res = self.llm.invoke(messages, **kwargs)
            latency_ms = (time.perf_counter() - start_t) * 1000
            self._log_llm_call(res, latency_ms)
            return res.content
        except Exception as e:
            for attempt in range(max_retries - 1):
                try:
                    time.sleep(retry_backoff_base ** attempt)
                    res = self.llm.invoke(messages, **kwargs)
                    return res.content
                except Exception:
                    pass
            raise e

    def complete_text_json(
        self,
        prompt: str,
        system_prompt: Optional[str] = None,
        temperature: float = None,
        max_tokens: int = None,
        max_retries: int = None,
        retry_backoff_base: int = None,
        allow_reasoning: bool = True,
    ) -> dict:
        """Text completion wrapper resolving into Dictionary structure natively."""
        temperature = temperature if temperature is not None else app_config.llm_default_temperature
        max_tokens = max_tokens if max_tokens is not None else app_config.llm_default_max_tokens
        max_retries = max_retries if max_retries is not None else app_config.llm_default_max_retries
        retry_backoff_base = retry_backoff_base if retry_backoff_base is not None else app_config.llm_retry_backoff_base
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
        raise ValueError(f"Failed to parse JSON after {max_retries} attempts: {last_error}")

    def structured_invoke(
        self,
        schema: type,
        prompt: str,
        system_prompt: Optional[str] = None,
        allow_reasoning: bool = True,
        max_retries: int = None,
        retry_backoff_base: int = None,
    ) -> any:
        from langchain_core.output_parsers import PydanticOutputParser
        from langchain_core.exceptions import OutputParserException
        
        max_retries = max_retries if max_retries is not None else app_config.llm_default_max_retries
        retry_backoff_base = retry_backoff_base if retry_backoff_base is not None else app_config.llm_retry_backoff_base
        
        parser = PydanticOutputParser(pydantic_object=schema)
        messages = []
        if system_prompt:
            messages.append(SystemMessage(content=system_prompt))
        messages.append(HumanMessage(content=prompt))

        # Try native structured output first
        if hasattr(self.llm, "with_structured_output"):
            try:
                llm_with_struct = self.llm.with_structured_output(schema)
                for attempt in range(max_retries):
                    try:
                        if attempt > 0:
                            time.sleep(retry_backoff_base ** attempt)
                        start_t = time.perf_counter()
                        result = llm_with_struct.invoke(messages)
                        logger.info("LLM Struct call | latency=%.0fms", (time.perf_counter() - start_t) * 1000)
                        return result
                    except Exception:
                        pass
            except NotImplementedError:
                pass
                
        # Graceful fallback: append formatting instructions & parse manually via Langchain core logic
        messages[-1] = HumanMessage(content=prompt + "\n\n" + parser.get_format_instructions())
        last_error = None
        for attempt in range(max_retries):
            try:
                if attempt > 0:
                    time.sleep(retry_backoff_base ** attempt)
                start_t = time.perf_counter()
                res = self.llm.invoke(messages)
                latency_ms = (time.perf_counter() - start_t) * 1000
                self._log_llm_call(res, latency_ms)
                return parser.invoke(res.content)
            except OutputParserException as e:
                last_error = e

        raise ValueError(f"Failed to extract structured schema after {max_retries} attempts: {last_error}")

    @staticmethod
    def _encode_image(image_path: str, max_dim: int = 512) -> str:
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

    @staticmethod
    def _parse_json_response(raw: str) -> dict:
        cleaned = raw.strip()
        if cleaned.startswith("```"):
            lines = cleaned.split("\n")
            if lines[0].startswith("```"):
                lines = lines[1:]
            if lines and lines[-1].strip() == "```":
                lines = lines[:-1]
            cleaned = "\n".join(lines)
        return json.loads(cleaned)
