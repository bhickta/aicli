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

def resolve_dynamic_model(preferred_string: str = None) -> str:
    """Fetch the first available model ID from LM Studio and explicitly load it via native API."""
    import json
    import urllib.request
    
    # Try native LM Studio v1 API first (lists all downloaded models)
    try:
        base_idx = config.lm_studio_base_url.rfind('/v1')
        if base_idx != -1:
            native_url = config.lm_studio_base_url[:base_idx] + "/api/v1/models"
            req = urllib.request.Request(native_url)
            model_to_load = None
            
            with urllib.request.urlopen(req, timeout=2) as resp:
                data = json.loads(resp.read())
                if "models" in data:
                    models = data["models"]
                    
                    # Sort so that preferred models bubble to the top
                    if preferred_string:
                        models = sorted(models, key=lambda x: 0 if preferred_string.lower() in x.get("key", "").lower() else 1)
                        
                    for m in models:
                        if m.get("type", "llm") == "llm":
                            # If it's already loaded, just return it! (Unless user prefers something else and it's not the top choice yet)
                            if len(m.get("loaded_instances", [])) > 0:
                                if not preferred_string or preferred_string.lower() in m.get("key", "").lower():
                                    return m["key"]
                                else:
                                    # We found A loaded model, but they specifically want another one. 
                                    pass
                                    
                            if not model_to_load:
                                model_to_load = m["key"] # store first available to load
                                
            # If nothing was explicitly loaded, force LM studio to load the first one
            if model_to_load:
                load_url = config.lm_studio_base_url[:base_idx] + "/api/v1/models/load"
                
                payload_dict = {"model": model_to_load}
                
                # Auto-apply the EXACT LM Studio advanced overrides requested for the 26B model
                if "26b" in model_to_load.lower():
                    payload_dict.update({
                        "context_length": 80549,
                        "gpu_offload": 30,
                        "cpu_thread_pool_size": 4,
                        "evaluation_batch_size": 512,
                        "max_concurrent_predictions": 4,
                        "unified_kv_cache": True,
                        "offload_kv_cache_to_gpu_memory": True,
                        "keep_model_in_memory": True,
                        "try_mmap": True,
                        "num_experts": 8
                    })
                    
                payload = json.dumps(payload_dict).encode('utf-8')
                load_req = urllib.request.Request(
                    load_url, 
                    data=payload, 
                    method="POST", 
                    headers={"Content-Type": "application/json"}
                )
                try:
                    # Heavy models (like 26B) take a long time to boot into VRAM, give it up to 3 minutes
                    urllib.request.urlopen(load_req, timeout=180) 
                except Exception:
                    pass # Proceed anyway and hope JIT catches it, or let the user see the 404
                return model_to_load
                
    except Exception:
        pass
        
    return config.model_name

def unload_all_models():
    """Unload all currently loaded models in LM Studio to free up VRAM."""
    import json
    import urllib.request
    
    try:
        base_idx = config.lm_studio_base_url.rfind('/v1')
        if base_idx == -1:
            return
            
        native_url = config.lm_studio_base_url[:base_idx] + "/api/v1/models"
        req = urllib.request.Request(native_url)
        
        models_to_unload = []
        with urllib.request.urlopen(req, timeout=2) as resp:
            data = json.loads(resp.read())
            if "models" in data:
                for m in data["models"]:
                    for instance in m.get("loaded_instances", []):
                        models_to_unload.append(instance.get("id"))
                        
        for instance_id in models_to_unload:
            if not instance_id:
                continue
            unload_url = config.lm_studio_base_url[:base_idx] + "/api/v1/models/unload"
            payload = json.dumps({"instance_id": instance_id}).encode('utf-8')
            unload_req = urllib.request.Request(
                unload_url, 
                data=payload, 
                method="POST", 
                headers={"Content-Type": "application/json"}
            )
            try:
                urllib.request.urlopen(unload_req, timeout=5)
            except Exception:
                pass
                
    except Exception:
        pass

