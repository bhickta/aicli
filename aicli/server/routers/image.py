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

class ImageCleanRequest(BaseModel):
    target_path: str
    auto_trash: bool = False
    strict: bool = False
    sync_refs: bool = False
    workers: int = 4

def _img_clean_worker(orch: BaseOrchestrator, req: ImageCleanRequest):
    import aicli.server.pipelines.image as img_mod
    orig_console = getattr(img_mod, "console", None)
    orig_print_success = getattr(img_mod, "print_success", None)
    try:
        img_mod.console = ConsoleRedirect(orch.queue)
        img_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        
        target = Path(req.target_path)
        if not target.is_absolute():
            target = ServerState.data_dir / req.target_path

        img_mod.clean_images(
            target_path=target,
            auto_trash=req.auto_trash,
            strict=req.strict,
            sync_refs=req.sync_refs,
            workers=req.workers
        )
    finally:
        if orig_console: img_mod.console = orig_console
        if orig_print_success: img_mod.print_success = orig_print_success

@router.post("/clean")
def run_img_clean(req: ImageCleanRequest):
    try:
        image_orch.dispatch(_img_clean_worker, req=req)
        return {"ok": True, "message": "Image Cleaner Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

class ImageDigitizeRequest(BaseModel):
    target_path: str
    auto_replace: bool = False
    sync_refs: bool = False
    workers: int = 2

def _img_digitize_worker(orch: BaseOrchestrator, req: ImageDigitizeRequest):
    import aicli.server.pipelines.image as img_mod
    orig_console = getattr(img_mod, "console", None)
    orig_print_success = getattr(img_mod, "print_success", None)
    try:
        img_mod.console = ConsoleRedirect(orch.queue)
        img_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        
        target = Path(req.target_path)
        if not target.is_absolute():
            target = ServerState.data_dir / req.target_path

        img_mod.digitize_images(
            target_path=target,
            auto_replace=req.auto_replace,
            sync_refs=req.sync_refs,
            workers=req.workers
        )
    finally:
        if orig_console: img_mod.console = orig_console
        if orig_print_success: img_mod.print_success = orig_print_success

@router.post("/digitize")
def run_img_digitize(req: ImageDigitizeRequest):
    try:
        image_orch.dispatch(_img_digitize_worker, req=req)
        return {"ok": True, "message": "Image Digitize Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))
