"""Configuration structures for AI CLI."""
from pydantic_settings import BaseSettings, SettingsConfigDict

class AppConfig(BaseSettings):
    """Configuration based on environment variables or .env file."""
    # LM Studio default settings
    lm_studio_base_url: str = "http://localhost:1234/v1"
    lm_studio_api_key: str = "sk-lm-UIfIMcJs:ga4Fhyit5WI6tz0FJTbR" # LM studio doesn't actually require one, but openai client expects it
    
    # Model to use, "local-model" usually works for LM studio
    model_name: str = "local-model"

    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8")

config = AppConfig()

def resolve_dynamic_model() -> str:
    """Fetch the first available model ID from LM Studio dynamically to trigger JIT loading."""
    import json
    import urllib.request
    
    # Try native LM Studio v1 API first (lists all downloaded models)
    try:
        base_idx = config.lm_studio_base_url.rfind('/v1')
        if base_idx != -1:
            native_url = config.lm_studio_base_url[:base_idx] + "/api/v1/models"
            req = urllib.request.Request(native_url)
            with urllib.request.urlopen(req, timeout=2) as resp:
                data = json.loads(resp.read())
                if data.get("data") and len(data["data"]) > 0:
                    return data["data"][0]["id"]
    except Exception:
        pass
        
    # Fallback to OpenAI compatible endpoint (lists loaded models)
    try:
        req = urllib.request.Request(f"{config.lm_studio_base_url}/models")
        with urllib.request.urlopen(req, timeout=2) as resp:
            data = json.loads(resp.read())
            if data.get("data") and len(data["data"]) > 0:
                return data["data"][0]["id"]
    except Exception:
        pass
        
    return config.model_name
