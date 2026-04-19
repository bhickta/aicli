"""Pydantic schemas for the News API."""
from typing import Optional

from pydantic import BaseModel


class NewsProcessRequestDTO(BaseModel):
    json_path: str
    output: Optional[str] = None
    workers: int = 4
    threshold: float = 0.8
    force_merge: bool = False
    no_cache: bool = False


class NewsDedupeRequestDTO(BaseModel):
    file_path: str
    output: Optional[str] = None
    workers: int = 10
    threshold: float = 0.8
