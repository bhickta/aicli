"""vLLM provider via LangChain."""
from aicli.config import config
from aicli.providers.base import LangChainProvider
from langchain_openai import ChatOpenAI

class VLLMProvider(LangChainProvider):
    """vLLM mapped natively as an OpenAI-compatible LangChain wrapper."""
    def __init__(self) -> None:
        base_url = config.vllm_base_url
        if not base_url.endswith("/v1"):
            base_url = f"{base_url.rstrip('/')}/v1"
            
        super().__init__(ChatOpenAI(
            base_url=base_url,
            api_key=config.vllm_api_key,
            model=config.model_name
        ))
