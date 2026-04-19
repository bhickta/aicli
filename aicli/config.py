"""Configuration structures for AI CLI."""

from pydantic_settings import BaseSettings, SettingsConfigDict

import os
import json
from pathlib import Path
from pydantic import BaseModel

CONFIG_DIR = Path.home() / ".config" / "aicli"
CONFIG_FILE = CONFIG_DIR / "settings.json"

PROVIDER_TYPE_CHOICES = ["ollama", "vllm"]


class AppConfig(BaseModel):
    """Configuration based on JSON settings."""

    provider_type: str = "ollama"
    ollama_base_url: str = "http://localhost:11434"
    ollama_api_key: str = "ollama"
    vllm_base_url: str = "http://localhost:8000"
    vllm_api_key: str = "EMPTY"
    model_name: str = "qwen3.5-9b"


def load_config() -> AppConfig:
    if CONFIG_FILE.exists():
        try:
            data = json.loads(CONFIG_FILE.read_text())
            return AppConfig(**data)
        except Exception:
            pass
    return AppConfig()


def save_config(cfg: AppConfig):
    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    CONFIG_FILE.write_text(cfg.model_dump_json(indent=2))


config = load_config()
