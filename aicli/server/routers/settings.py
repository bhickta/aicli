from fastapi import APIRouter
from aicli.config import config, load_config, save_config, AppConfig

router = APIRouter()

@router.get("")
def get_settings():
    return config.model_dump()

@router.post("")
def update_settings(new_config: AppConfig):
    global config
    # Update the global object fields so other modules get the new values
    config.lm_studio_base_url = new_config.lm_studio_base_url
    config.lm_studio_api_key = new_config.lm_studio_api_key
    config.model_name = new_config.model_name
    
    # Save it to disk
    save_config(config)
    
    return {"ok": True, "message": "Settings updated successfully."}
