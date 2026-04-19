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
    # Dynamically update all fields in existing config from new_config
    for key, value in new_config.model_dump().items():
        setattr(config, key, value)

    save_config(config)

    return {"ok": True, "message": "Settings updated successfully."}
