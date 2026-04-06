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
        """Helper to read an image file and encode it as a base64 string."""
        with open(image_path, "rb") as image_file:
            return base64.b64encode(image_file.read()).decode("utf-8")
            
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
        
        messages = []
        if system_prompt:
            messages.append({"role": "system", "content": system_prompt})
            
        messages.append({
            "role": "user",
            "content": [
                {"type": "text", "text": prompt},
                {
                    "type": "image_url",
                    "image_url": {
                        "url": f"data:{mime_type};base64,{base64_image}"
                    }
                }
            ]
        })
        
        response = self.client.chat.completions.create(
            model=config.model_name,
            messages=messages,
            temperature=0.2, # Lower temperature for better accuracy/formatting
            max_tokens=60
        )
        
        return response.choices[0].message.content.strip()
