"""Core interfaces and abstractions (SOLID/Open-Closed)."""
from abc import ABC, abstractmethod

class ImageVisionProvider(ABC):
    """Abstract base class for any provider that can look at an image or handle text logic."""

    @abstractmethod
    def describe_image(self, image_path: str, prompt: str, system_prompt: str = None, 
                       allow_reasoning: bool = True) -> str:
        """Takes an image and a prompt, returns text."""
        pass

    @abstractmethod
    def complete_text(self, prompt: str, system_prompt: str = None, 
                      allow_reasoning: bool = True) -> str:
        """Standard text-only completion."""
        pass

    @abstractmethod
    def complete_text_json(self, prompt: str, system_prompt: str = None, 
                           allow_reasoning: bool = True) -> dict:
        """Text completion that returns a parsed JSON object."""
        pass

    @abstractmethod
    def structured_invoke(self, schema: type, prompt: str, system_prompt: str = None,
                          allow_reasoning: bool = True) -> any:
        """Text completion that returns a strongly typed Pydantic schema using structured outputs."""
        pass
