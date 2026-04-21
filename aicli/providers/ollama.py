import subprocess
import shutil
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

    @staticmethod
    def list_models() -> list[str]:
        """Returns a list of available models from Ollama."""
        if not shutil.which("ollama"):
            return []
        try:
            res = subprocess.run(["ollama", "list"], capture_output=True, text=True)
            models = []
            lines = res.stdout.splitlines()
            if len(lines) > 1:
                for line in lines[1:]: # Skip header
                    if line.strip():
                        models.append(line.split()[0].strip())
            return models
        except Exception:
            return []
