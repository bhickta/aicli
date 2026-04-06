import os
import re
from pathlib import Path
from aicli.core.interfaces import ImageVisionProvider

class ImageRenamerService:
    """
    High-level service that coordinates with an ImageVisionProvider to generate a new name for an image,
    and applies the rename operation securely.
    """
    
    def __init__(self, ai_provider: ImageVisionProvider):
        # We inject the provider here (Dependency Injection) adhering to SOLID principles
        self.provider = ai_provider
        
    def generate_new_name(self, image_path: str) -> str:
        """
        Uses the vision provider to look at the image and suggest a new concise filename (without extension).
        """
        prompt = (
            "You are a specialized file-renaming AI. Based on the image content, generate a very short, "
            "highly descriptive file name in kebab-case.\n"
            "CRITICAL RULES:\n"
            "- Output ONLY the raw kebab-case string.\n"
            "- NO introductory text, NO markdown, NO quotes, NO file extensions.\n"
            "Example valid output: vintage-red-sports-car"
        )
        
        suggested_name = self.provider.describe_image(image_path, prompt)
        
        # Aggressive post-processing for local LLMs that are "chatty"
        cleaned = suggested_name.replace("`", "").replace('"', "").replace("'", "").strip()
        
        if " " in cleaned or "\n" in cleaned:
            # If the model outputs a sentence (e.g. "Here is the name: red-car"), 
            # find the longest kebab-case word or fallback to the last word.
            words = cleaned.split()
            kebab_words = [w for w in words if "-" in w]
            if kebab_words:
                cleaned = sorted(kebab_words, key=len)[-1]
            else:
                cleaned = "-".join(words[-3:]).lower() # smash last 3 words into kebab-case
                
        # Strip trailing punctuation like a period if the LLM wrote a sentence
        cleaned = re.sub(r'[^a-zA-Z0-9-]', '', cleaned)
        return cleaned

    def apply_rename(self, image_path: str, new_name_without_ext: str) -> str:
        """
        Renames the file on disk and returns the new absolute path.
        """
        path_obj = Path(image_path)
        if not path_obj.exists():
            raise FileNotFoundError(f"Image {image_path} does not exist.")
            
        ext = path_obj.suffix
        new_filename = f"{new_name_without_ext}{ext}"
        new_path = path_obj.parent / new_filename
        
        # Don't overwrite existing files
        if new_path.exists():
            raise FileExistsError(f"Cannot rename to {new_filename} because that file already exists.")
            
        os.rename(path_obj, new_path)
        return str(new_path)
