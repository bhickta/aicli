"""Resolves per-step reasoning flags for the pipeline."""
from typing import Dict, Optional

from aicli.server.constants.analyze_constants import RECOMMENDED_REASONING


class ReasoningResolver:
    """Determines whether a given pipeline step should use deep reasoning.

    Encapsulates the priority chain:
      1. If the master `allow_reasoning` toggle is off → always False
      2. If the caller provided an explicit per-step override → use it
      3. Otherwise → fall back to the recommended defaults
    """

    def __init__(
        self,
        allow_reasoning: bool = True,
        step_overrides: Optional[Dict[str, bool]] = None,
    ) -> None:
        self._allow_reasoning = allow_reasoning
        self._step_overrides = step_overrides or {}

    def should_think(self, step_id: int) -> bool:
        """Return True if the given step should use reasoning."""
        if not self._allow_reasoning:
            return False
        override = self._resolve_override(step_id)
        if override is not None:
            return override
        return RECOMMENDED_REASONING.get(step_id, False)

    def _resolve_override(self, step_id: int) -> Optional[bool]:
        """Check both string and int keys from JSON/Frontend payloads."""
        val = self._step_overrides.get(str(step_id))
        if val is None:
            val = self._step_overrides.get(step_id)
        return bool(val) if val is not None else None
