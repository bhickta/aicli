import subprocess
import shutil
import logging
from aicli.config import config
from aicli.providers.base import LangChainProvider
from langchain_openai import ChatOpenAI

logger = logging.getLogger(__name__)

class LMStudioProvider(LangChainProvider):
    """LM Studio mapped natively as an OpenAI-compatible LangChain wrapper with Auto-Loading."""
    def __init__(self) -> None:
        model_name = config.model_name
        self._ensure_model_loaded(model_name)
        
        super().__init__(ChatOpenAI(
            base_url=config.lm_studio_base_url,
            api_key=config.lm_studio_api_key,
            model=model_name
        ))

    @staticmethod
    def list_models() -> list[str]:
        """Returns a list of available model identifiers from LM Studio."""
        if not shutil.which("lms"):
            return []
        try:
            res = subprocess.run(["lms", "ls"], capture_output=True, text=True)
            models = []
            for line in res.stdout.splitlines():
                if "/" in line:
                    models.append(line.split()[0].strip())
            return models
        except Exception:
            return []

    def _ensure_model_loaded(self, model_name: str):
        """Uses 'lms' CLI to verify if a model is loaded; if not, attempts to load it."""
        if not shutil.which("lms"):
            return

        try:
            # 1. Check currently loaded models
            ps_res = subprocess.run(["lms", "ps"], capture_output=True, text=True)
            if model_name in ps_res.stdout:
                return # Already loaded

            # 2. Find full identifier if only a partial name was provided
            # e.g. "gemma-4-e4b" -> "google/gemma-4-e4b"
            ls_res = subprocess.run(["lms", "ls"], capture_output=True, text=True)
            full_identifier = model_name
            for line in ls_res.stdout.splitlines():
                if model_name in line and "/" in line:
                    full_identifier = line.split()[0].strip()
                    break

            # 3. Attempt load
            logger.info(f"LM Studio: Auto-loading model '{full_identifier}'...")
            subprocess.run(["lms", "load", full_identifier], check=True, capture_output=True)
            logger.info(f"LM Studio: Successfully loaded '{full_identifier}'")
        except Exception as e:
            logger.warning(f"LM Studio: Auto-load failed for '{model_name}': {e}")
