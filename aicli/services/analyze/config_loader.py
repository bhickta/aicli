"""Configuration loader for the UPSC analyze pipeline.

Reads prompts.yaml from the domains/analyze directory and provides
typed access to prompts, dimension configs, and LM Studio settings.
"""
from pathlib import Path

import yaml


# Default config path: aicli/domains/analyze/prompts.yaml
_DEFAULT_CONFIG_PATH = Path(__file__).resolve().parent.parent.parent / "domains" / "analyze" / "prompts.yaml"


class AnalyzeConfig:
    """Loads prompts.yaml and provides typed access to all pipeline configuration."""

    def __init__(self, config_path: Path | None = None):
        self._path = config_path or _DEFAULT_CONFIG_PATH
        self._data: dict = {}
        self.reload()

    def reload(self):
        """(Re)load the YAML config from disk."""
        with open(self._path, "r", encoding="utf-8") as f:
            self._data = yaml.safe_load(f)

    # ------------------------------------------------------------------
    # LM Studio settings
    # ------------------------------------------------------------------
    @property
    def lm_settings(self) -> dict:
        return self._data.get("lm_studio", {})

    @property
    def max_tokens(self) -> int:
        return self.lm_settings.get("max_tokens", 2000)

    @property
    def temperature(self) -> float:
        return self.lm_settings.get("temperature", 0.1)

    @property
    def max_retries(self) -> int:
        return self.lm_settings.get("max_retries", 3)

    @property
    def retry_backoff_base(self) -> int:
        return self.lm_settings.get("retry_backoff_base", 2)

    @property
    def image_max_size(self) -> int:
        return self.lm_settings.get("image_max_size", 1024)

    # ------------------------------------------------------------------
    # Step prompts
    # ------------------------------------------------------------------
    @property
    def classification_prompt(self) -> str:
        return self._data["classification"]["prompt"]

    @property
    def transcription_prompt(self) -> str:
        return self._data["transcription"]["prompt"]

    @property
    def segmentation_prompt(self) -> str:
        return self._data["segmentation"]["prompt"]

    @property
    def aggregation_prompt_template(self) -> str:
        return self._data["aggregation"]["prompt"]

    @property
    def metadata_prompt(self) -> str:
        return self._data["metadata"]["prompt"]

    # ------------------------------------------------------------------
    # Dimensions
    # ------------------------------------------------------------------
    @property
    def all_dimensions(self) -> dict[str, dict]:
        """Return the full dimensions dict from config."""
        return self._data.get("dimensions", {})

    @property
    def enabled_dimensions(self) -> list[str]:
        """Return names of all enabled dimensions."""
        return [
            name
            for name, cfg in self.all_dimensions.items()
            if cfg.get("enabled", True)
        ]

    def get_dimension_prompt(self, name: str) -> str:
        """Get the prompt template for a specific dimension."""
        dim = self.all_dimensions.get(name)
        if not dim:
            raise ValueError(f"Unknown dimension: '{name}'. Available: {list(self.all_dimensions.keys())}")
        return dim["prompt"]

    def is_dimension_enabled(self, name: str) -> bool:
        """Check if a dimension is enabled."""
        dim = self.all_dimensions.get(name)
        return dim.get("enabled", True) if dim else False
