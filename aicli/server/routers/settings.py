from fastapi import APIRouter
from aicli.config import (
    config,
    load_config,
    save_config,
    AppConfig,
    PROVIDER_TYPE_CHOICES,
)

router = APIRouter()


@router.get("")
def get_settings():
    return config.model_dump()


@router.get("/providers")
def get_available_providers():
    return {"providers": PROVIDER_TYPE_CHOICES}


@router.post("")
def update_settings(new_config: AppConfig):
    global config
    config.provider_type = new_config.provider_type
    config.ollama_base_url = new_config.ollama_base_url
    config.ollama_api_key = new_config.ollama_api_key
    config.vllm_base_url = new_config.vllm_base_url
    config.vllm_api_key = new_config.vllm_api_key
    config.model_name = new_config.model_name

    save_config(config)

    return {"ok": True, "message": "Settings updated successfully."}
