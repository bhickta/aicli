"""Provider implementations."""

from aicli.config import config, PROVIDER_TYPE_CHOICES
from aicli.core.interfaces import ImageVisionProvider


def get_provider() -> ImageVisionProvider:
    """Factory function to get the configured provider."""
    from aicli.providers.ollama import OllamaProvider
    from aicli.providers.vllm import VLLMProvider

    provider_type = config.provider_type.lower()
    if provider_type == "vllm":
        return VLLMProvider()
    return OllamaProvider()


__all__ = ["get_provider", "ImageVisionProvider"]
