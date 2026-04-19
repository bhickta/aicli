"""LM Studio provider via LangChain."""
from aicli.config import config
from aicli.providers.base import LangChainProvider
from langchain_openai import ChatOpenAI

class LMStudioProvider(LangChainProvider):
    """LM Studio mapped natively as an OpenAI-compatible LangChain wrapper."""
    def __init__(self) -> None:
        super().__init__(ChatOpenAI(
            base_url=config.lm_studio_base_url,
            api_key=config.lm_studio_api_key,
            model=config.model_name
        ))
