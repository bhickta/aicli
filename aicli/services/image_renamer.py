import os
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
            "Look at this image and suggest a very short, descriptive file name in kebab-case "
            "for it. Just return the file name with no extension, no markdown, no quotes, "
            "and nothing else."
        )
        
        suggested_name = self.provider.describe_image(image_path, prompt)
        
        # Clean up any potential markdown or quotes the AI might still add
        cleaned = suggested_name.replace("`", "").replace('"', "").replace("'", "").strip()
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
