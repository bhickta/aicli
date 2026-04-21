"""Pydantic schemas for the Video API."""
from typing import Optional

from pydantic import BaseModel


class VideoCourseRequestDTO(BaseModel):
    target_dir: str
    whisper_model: str = "large-v3"
    cleanup: str = "trash"
    w1: int = 1
    w2: int = 12
    w3: int = 4
    llm_model: str = "gemma-4-e4b"
    max_merge_hours: float = 11.0


class VideoCompressRequestDTO(BaseModel):
    target_path: str
    resolution: int = 240
    preset: str = "light"
    overwrite: bool = False
    workers: int = 4
    crf: Optional[int] = None
    fps: Optional[str] = None
    fast_skip: bool = False


class VideoTagRequestDTO(BaseModel):
    target_path: str
    write: bool = False
    no_rename: bool = False
    full_cc: bool = False
    text_thumb: bool = True
    retranscribe: bool = False
    transcribe_only: bool = False
    workers: int = 2
    clip_every: int = 360
    clip_len: int = 60
    save_txt: bool = False
    whisper_model: str = "base"


class VideoNotesRequestDTO(BaseModel):
    target_path: str
    overwrite: bool = False
    style: str = "bullet"
