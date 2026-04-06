import base64
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
    
    def _encode_image_to_base64(self, image_path: str) -> str:
        """Helper to read an image file, safely compress/resize it, and encode as base64."""
        from PIL import Image
        import io
        
        with Image.open(image_path) as img:
            # Convert to RGB to strip alpha channels which some VLMs hate in JPEGs
            if img.mode in ("RGBA", "P"):
                img = img.convert("RGB")
                
            # Scale down large images (e.g. 4K pages) to max 512x512
            # Even 1024x1024 parses into ~4000 tokens in some VLMs, which exceeds LM Studio's 
            # default 2048 context window and instantly triggers a 'length' rejection.
            # 512px guarantees it fits in context while keeping prominent text perfectly legible.
            max_size = 512
            if max(img.size) > max_size:
                img.thumbnail((max_size, max_size), Image.Resampling.LANCZOS)
                
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

    def describe_image(self, image_path: str, prompt: str, system_prompt: str = None) -> str:
        """
        Sends the base64 encoded image and the prompt to LM Studio.
        """
        base64_image = self._encode_image_to_base64(image_path)
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
        
        response = self.client.chat.completions.create(
            model=config.model_name,
            messages=messages,
            temperature=0.2 # Lower temperature for better accuracy/formatting
        )
        
        content = response.choices[0].message.content
        if not content:
            # If the VLM crashes or fails to generate text, capture the internal API reason
            reason = response.choices[0].finish_reason
            raise ValueError(f"LM Studio aborted generation (finish_reason: '{reason}'). Image may be too complex, or context window exceeded.")
            
        return content.strip()
