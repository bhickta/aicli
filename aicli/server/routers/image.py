"""FastAPI router for Image pipelines (Rename, Clean, Digitize)."""
from pathlib import Path

from fastapi import APIRouter, HTTPException
from fastapi.responses import EventSourceResponse

from aicli.server.orchestrator.base import BaseOrchestrator
from aicli.server.orchestrator.console_patcher import ConsolePatcher
from aicli.server.dependencies import ServerState
from aicli.server.schemas.image_schemas import (
    ImageRenameRequestDTO,
    ImageCleanRequestDTO,
    ImageDigitizeRequestDTO,
)

router = APIRouter()
image_orch = BaseOrchestrator()


# ── Workers ─────────────────────────────────────────────────────────

def _img_rename_worker(orch: BaseOrchestrator, req: ImageRenameRequestDTO) -> None:
    import aicli.server.pipelines.image as img_mod

    target = _resolve_path(req.target_path)
    with ConsolePatcher(img_mod, orch.queue):
        img_mod.rename_image(
            target_path=target,
            auto_rename=req.auto_rename,
            workers=req.workers,
            sync_refs=req.sync_refs,
            trash_junk=req.trash_junk,
        )


def _img_clean_worker(orch: BaseOrchestrator, req: ImageCleanRequestDTO) -> None:
    import aicli.server.pipelines.image as img_mod

    target = _resolve_path(req.target_path)
    with ConsolePatcher(img_mod, orch.queue):
        img_mod.clean_images(
            target_path=target,
            auto_trash=req.auto_trash,
            strict=req.strict,
            sync_refs=req.sync_refs,
            workers=req.workers,
        )


def _img_digitize_worker(orch: BaseOrchestrator, req: ImageDigitizeRequestDTO) -> None:
    import aicli.server.pipelines.image as img_mod

    target = _resolve_path(req.target_path)
    with ConsolePatcher(img_mod, orch.queue):
        img_mod.digitize_images(
            target_path=target,
            auto_replace=req.auto_replace,
            sync_refs=req.sync_refs,
            workers=req.workers,
        )


# ── Endpoints ───────────────────────────────────────────────────────

@router.post("/rename")
def run_img_rename(req: ImageRenameRequestDTO):
    return _dispatch(image_orch, _img_rename_worker, "Image Rename Pipeline", req=req)


@router.post("/clean")
def run_img_clean(req: ImageCleanRequestDTO):
    return _dispatch(image_orch, _img_clean_worker, "Image Cleaner Pipeline", req=req)


@router.post("/digitize")
def run_img_digitize(req: ImageDigitizeRequestDTO):
    return _dispatch(image_orch, _img_digitize_worker, "Image Digitize Pipeline", req=req)


@router.get("/stream")
async def stream_image():
    return EventSourceResponse(image_orch.stream_events())


# ── Helpers ─────────────────────────────────────────────────────────

def _resolve_path(raw_path: str) -> Path:
    """Resolve a path, making relative paths relative to data_dir."""
    target = Path(raw_path)
    if not target.is_absolute():
        target = ServerState.data_dir / raw_path
    return target


def _dispatch(orch: BaseOrchestrator, worker, pipeline_name: str, **kwargs):
    """Common dispatch-and-respond pattern for all pipeline endpoints."""
    try:
        orch.dispatch(worker, **kwargs)
        return {"ok": True, "message": f"{pipeline_name} started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))
