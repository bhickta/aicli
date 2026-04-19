"""Pydantic schemas for the Image API."""
from pydantic import BaseModel


class ImageRenameRequestDTO(BaseModel):
    target_path: str
    auto_rename: bool = False
    workers: int = 4
    sync_refs: bool = False
    trash_junk: bool = False


class ImageCleanRequestDTO(BaseModel):
    target_path: str
    auto_trash: bool = False
    strict: bool = False
    sync_refs: bool = False
    workers: int = 4


class ImageDigitizeRequestDTO(BaseModel):
    target_path: str
    auto_replace: bool = False
    sync_refs: bool = False
    workers: int = 2
