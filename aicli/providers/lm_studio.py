import base64
import json
import time

from openai import OpenAI
from aicli.core.interfaces import ImageVisionProvider
from aicli.config import config

class LMStudioProvider(ImageVisionProvider):
    """Implementation of ImageVisionProvider using LM Studio via the OpenAI SDK."""
    
    def __init__(self):
        # We point the client precisely to the LM Studio base URL
        self.client = OpenAI(
            base_url=config.lm_studio_base_url,
            api_key=config.lm_studio_api_key
        )
    
    def _encode_image_to_base64(self, image_path: str, max_dim: int = 512) -> str:
        """Helper to read an image file, safely compress/resize it, and encode as base64."""
        from PIL import Image
        import io
        
        with Image.open(image_path) as img:
            # Convert to RGB to strip alpha channels which some VLMs hate in JPEGs
            if img.mode in ("RGBA", "P"):
                img = img.convert("RGB")
                
            # Scale down large images to fit in context window.
            # Default 512px for general use; 1024px for handwriting legibility.
            if max(img.size) > max_dim:
                img.thumbnail((max_dim, max_dim), Image.Resampling.LANCZOS)
                
            buffer = io.BytesIO()
            img.save(buffer, format="JPEG", quality=85)
            return base64.b64encode(buffer.getvalue()).decode("utf-8")
            
    def _get_mime_type(self, image_path: str) -> str:
        """Simple helper to guess mime type from extension."""
        ext = image_path.lower().split('.')[-1]
        if ext in ['jpg', 'jpeg']:
            return "image/jpeg"
        elif ext == "png":
            return "image/png"
        elif ext == "webp":
            return "image/webp"
        elif ext == "gif":
            return "image/gif"
        return "image/jpeg"

    def describe_image(self, image_path: str, prompt: str, system_prompt: str = None,
                       max_size: int = 512, temperature: float = 0.2,
                       max_tokens: int = None, max_retries: int = 1,
                       retry_backoff_base: int = 2, allow_reasoning: bool = True) -> str:
        """
        Sends the base64 encoded image and the prompt to LM Studio.
        
        Args:
            max_size: Max image dimension in pixels. 512 for general, 1024 for handwriting.
            max_retries: Number of retry attempts on failure.
            retry_backoff_base: Base for exponential backoff between retries.
        """
        base64_image = self._encode_image_to_base64(image_path, max_dim=max_size)
        mime_type = self._get_mime_type(image_path)
        
        # Many local Vision models crash or return empty strings if a "system" role is passed 
        # alongside an image. The safest approach is to collapse the system instructions 
        # directly into the standard "user" payload.
        combined_text = f"{system_prompt}\n\n{prompt}" if system_prompt else prompt
            
        messages = [
            {
                "role": "user",
                "content": [
                    {"type": "text", "text": combined_text},
                    {
                        "type": "image_url",
                        "image_url": {
                            "url": f"data:{mime_type};base64,{base64_image}"
                        }
                    }
                ]
            }
        ]
        
        create_kwargs = {
            "model": config.model_name,
            "messages": messages,
            "temperature": temperature,
        }
        if max_tokens:
            create_kwargs["max_tokens"] = max_tokens

        last_error = None
        for attempt in range(max_retries):
            try:
                response = self.client.chat.completions.create(**create_kwargs)
                content = response.choices[0].message.content
                if content:
                    return content.strip()
                reason = response.choices[0].finish_reason
                raise ValueError(f"LM Studio returned empty content (finish_reason: '{reason}'). Context window may be exceeded.")
            except Exception as e:
                last_error = e
                if attempt < max_retries - 1:
                    wait = retry_backoff_base ** attempt
                    time.sleep(wait)
        
        raise last_error

    def complete_text(self, prompt: str, system_prompt: str = None,
                      temperature: float = 0.1, max_tokens: int = 2000,
                      max_retries: int = 1, retry_backoff_base: int = 2,
                      allow_reasoning: bool = True) -> str:
        """
        Text-only completion via LM Studio (no image). Used for analysis,
        segmentation, and aggregation steps.
        """
        messages = []
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})
        messages.append({"role": "user", "content": prompt})
        
        create_kwargs = {
            "model": config.model_name,
            "messages": messages,
            "temperature": temperature,
            "max_tokens": max_tokens,
        }

        last_error = None
        for attempt in range(max_retries):
            # Attempt with reasoning control first
            try:
                # Add reasoning control to extra_body if provided
                current_kwargs = create_kwargs.copy()
                current_kwargs["extra_body"] = {"reasoning": allow_reasoning}
                
                response = self.client.chat.completions.create(**current_kwargs)
                content = response.choices[0].message.content
                if not content:
                    reason = response.choices[0].finish_reason
                    raise ValueError(f"LM Studio aborted generation (finish_reason: '{reason}').")
                
                # Sanitize: Strip thought/think blocks that might leak into content
                import re
                content = re.sub(r'<\|channel\|>thought.*?<\|channel\|>', '', content, flags=re.DOTALL)
                content = re.sub(r'<\|channel\|>thought.*', '', content, flags=re.DOTALL)
                content = re.sub(r'<think>.*?</think>', '', content, flags=re.DOTALL)
                content = re.sub(r'<thought>.*?</thought>', '', content, flags=re.DOTALL)
                
                return content.strip()
            except Exception as e:
                # If we get a 400 error, it might be the 'reasoning' parameter being rejected
                # Fallback to standard call without extra_body
                if "400" in str(e):
                    try:
                        response = self.client.chat.completions.create(**create_kwargs)
                        content = response.choices[0].message.content
                        if content: return content.strip()
                    except: pass
                last_error = e
                if attempt < max_retries - 1:
                    wait = retry_backoff_base ** attempt
                    time.sleep(wait)
        
        raise last_error

    def complete_text_json(self, prompt: str, system_prompt: str = None,
                           temperature: float = 0.1, max_tokens: int = 2000,
                           max_retries: int = 3, retry_backoff_base: int = 2,
                           allow_reasoning: bool = True) -> dict:
        """
        Text completion that parses the response as JSON. Retries with a
        stricter prompt prefix on parse failure.
        """
        last_error = None
        for attempt in range(max_retries):
            try:
                prefix = ""
                if attempt > 0:
                    prefix = "IMPORTANT: Return ONLY valid JSON, no markdown fences, no explanation.\n\n"
                
                raw = self.complete_text(
                    prompt=prefix + prompt,
                    system_prompt=system_prompt,
                    temperature=temperature,
                    max_tokens=max_tokens,
                    max_retries=1,
                    allow_reasoning=allow_reasoning,
                )
                
                # Strip markdown code fences if present
                cleaned = raw.strip()
                if cleaned.startswith("```"):
                    lines = cleaned.split("\n")
                    # Remove first and last fence lines
                    if lines[0].startswith("```"):
                        lines = lines[1:]
                    if lines and lines[-1].strip() == "```":
                        lines = lines[:-1]
                    cleaned = "\n".join(lines)
                
                return json.loads(cleaned)
            except (json.JSONDecodeError, ValueError) as e:
                last_error = e
                if attempt < max_retries - 1:
                    wait = retry_backoff_base ** attempt
                    time.sleep(wait)
        
        raise ValueError(f"Failed to parse JSON after {max_retries} attempts: {last_error}")
