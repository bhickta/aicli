"""OpenAI provider via LangChain."""
from aicli.config import config
from aicli.providers.base import LangChainProvider
from langchain_openai import ChatOpenAI

class OpenAIProvider(LangChainProvider):
    """OpenAI provider initialized natively under Langchain."""
    def __init__(self) -> None:
        super().__init__(ChatOpenAI(
            api_key=config.openai_api_key,
            model=config.model_name
        ))
