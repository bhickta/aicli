"""Anthropic provider via LangChain."""
from aicli.config import config
from aicli.providers.base import LangChainProvider
from langchain_anthropic import ChatAnthropic

class AnthropicProvider(LangChainProvider):
    """Anthropic provider initialized natively under Langchain."""
    def __init__(self) -> None:
        super().__init__(ChatAnthropic(
            api_key=config.anthropic_api_key,
            model=config.model_name
        ))
