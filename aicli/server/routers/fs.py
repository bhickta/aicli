from pathlib import Path
from typing import List, Dict, Any
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

router = APIRouter()

class FSItem(BaseModel):
    name: str
    path: str
    is_dir: bool
    size: int = 0

@router.get("/list")
def list_directory(path: str = "/"):
    """List contents of a directory for the file explorer."""
    try:
        p = Path(path).expanduser().resolve()
        if not p.exists():
            # Try relative to home if absolute fails
            p = Path.home() / path.lstrip("/")
            
        if not p.is_dir():
            raise HTTPException(status_code=400, detail="Path is not a directory")
            
        items = []
        # Sort so directories appear first
        for item in sorted(p.iterdir(), key=lambda x: (not x.is_dir(), x.name.lower())):
            # Skip hidden files
            if item.name.startswith("."):
                continue
                
            is_dir = item.is_dir()
            items.append(FSItem(
                name=item.name,
                path=str(item.absolute()),
                is_dir=is_dir,
                size=item.stat().st_size if not is_dir else 0
            ))
        return {"items": items, "parent": str(p.parent), "current": str(p)}
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))

@router.get("/home")
def get_home_dir():
    """Get the user's home directory."""
    return {"path": str(Path.home())}
