"""Provider implementations."""

from aicli.config import config, PROVIDER_TYPE_CHOICES
from aicli.core.interfaces import ImageVisionProvider


def get_provider() -> ImageVisionProvider:
    """Factory function to get the configured provider."""
    from aicli.providers.ollama import OllamaProvider
    from aicli.providers.vllm import VLLMProvider
    from aicli.providers.lm_studio import LMStudioProvider
    from aicli.providers.openai_provider import OpenAIProvider
    from aicli.providers.anthropic_provider import AnthropicProvider
    from aicli.providers.gemini_provider import GeminiProvider

    provider_registry = {
        "vllm": VLLMProvider,
        "lmstudio": LMStudioProvider,
        "openai": OpenAIProvider,
        "anthropic": AnthropicProvider,
        "gemini": GeminiProvider,
        "ollama": OllamaProvider
    }

    provider_class = provider_registry.get(config.provider_type.lower(), OllamaProvider)
    return provider_class()


__all__ = ["get_provider", "ImageVisionProvider"]
