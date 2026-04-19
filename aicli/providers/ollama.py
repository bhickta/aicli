"""Ollama provider via LangChain."""
from aicli.config import config
from aicli.providers.base import LangChainProvider
from langchain_ollama import ChatOllama

class OllamaProvider(LangChainProvider):
    """Ollama provider initialized natively under Langchain."""
    def __init__(self) -> None:
        super().__init__(ChatOllama(
            base_url=config.ollama_base_url,
            model=config.model_name
        ))
