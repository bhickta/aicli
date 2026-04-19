"""Gemini provider via LangChain."""
from aicli.config import config
from aicli.providers.base import LangChainProvider
from langchain_google_genai import ChatGoogleGenerativeAI

class GeminiProvider(LangChainProvider):
    """Gemini provider initialized natively under Langchain."""
    def __init__(self) -> None:
        super().__init__(ChatGoogleGenerativeAI(
            api_key=config.gemini_api_key,
            model=config.model_name
        ))
