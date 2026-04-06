"""Core interfaces and abstractions (SOLID/Open-Closed)."""
from abc import ABC, abstractmethod

class ImageVisionProvider(ABC):
    """Abstract base class for any provider that can look at an image and return text."""

    @abstractmethod
    def describe_image(self, image_path: str, prompt: str, system_prompt: str = None) -> str:
        """
        Takes an image path and a prompt, uses the provider's vision model, 
        and returns the resulting text.
        """
        pass
