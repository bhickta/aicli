import urllib.parse
from pathlib import Path

from fastapi import APIRouter, HTTPException
from fastapi.responses import EventSourceResponse
from pydantic import BaseModel

from aicli.server.orchestrator.base import BaseOrchestrator, SSEProgressContext, ConsoleRedirect

router = APIRouter()

# Share ServerState
from aicli.server.routers.analyze import ServerState

image_orch = BaseOrchestrator()

class ImageRenameRequest(BaseModel):
    target_path: str
    auto_rename: bool = False
    workers: int = 4
    sync_refs: bool = False
    trash_junk: bool = False

def _img_rename_worker(orch: BaseOrchestrator, req: ImageRenameRequest):
    import aicli.server.pipelines.image as img_mod
    orig_console = getattr(img_mod, "console", None)
    orig_print_success = getattr(img_mod, "print_success", None)
    try:
        img_mod.console = ConsoleRedirect(orch.queue)
        img_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        
        target = Path(req.target_path)
        if not target.is_absolute():
            target = ServerState.data_dir / req.target_path

        img_mod.rename_image(
            target_path=target,
            auto_rename=req.auto_rename,
            workers=req.workers,
            sync_refs=req.sync_refs,
            trash_junk=req.trash_junk
        )
    finally:
        if orig_console: img_mod.console = orig_console
        if orig_print_success: img_mod.print_success = orig_print_success


@router.post("/rename")
def run_img_rename(req: ImageRenameRequest):
    try:
        image_orch.dispatch(_img_rename_worker, req=req)
        return {"ok": True, "message": "Image Rename Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

@router.get("/stream")
async def stream_image():
    return EventSourceResponse(image_orch.stream_events())
