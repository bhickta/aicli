"""FastAPI router for News pipelines (Process, Dedupe)."""
from pathlib import Path

from fastapi import APIRouter, HTTPException
from fastapi.responses import EventSourceResponse

from aicli.server.orchestrator.base import BaseOrchestrator
from aicli.server.orchestrator.console_patcher import ConsolePatcher
from aicli.server.dependencies import ServerState
from aicli.server.schemas.news_schemas import (
    NewsProcessRequestDTO,
    NewsDedupeRequestDTO,
)

router = APIRouter()
news_orch = BaseOrchestrator()


# ── Workers ─────────────────────────────────────────────────────────

def _news_process_worker(orch: BaseOrchestrator, req: NewsProcessRequestDTO) -> None:
    import aicli.server.pipelines.news as news_mod

    json_file = _resolve_path(req.json_path)
    out_file = _resolve_path(req.output) if req.output else None

    with ConsolePatcher(news_mod, orch.queue):
        news_mod.process_news(
            json_path=json_file,
            output=out_file,
            workers=req.workers,
            threshold=req.threshold,
            force_merge=req.force_merge,
            no_cache=req.no_cache,
        )


def _news_dedupe_worker(orch: BaseOrchestrator, req: NewsDedupeRequestDTO) -> None:
    import aicli.server.pipelines.news as news_mod

    excel_file = _resolve_path(req.file_path)
    out_file = _resolve_path(req.output) if req.output else None

    with ConsolePatcher(news_mod, orch.queue):
        news_mod.dedupe(
            file_path=excel_file,
            output=out_file,
            threshold=req.threshold,
            workers=req.workers,
        )


# ── Endpoints ───────────────────────────────────────────────────────

@router.post("/process")
def run_news_process(req: NewsProcessRequestDTO):
    return _dispatch(news_orch, _news_process_worker, "News Process Pipeline", req=req)


@router.post("/dedupe")
def run_news_dedupe(req: NewsDedupeRequestDTO):
    return _dispatch(news_orch, _news_dedupe_worker, "News Dedupe Pipeline", req=req)


@router.get("/stream")
async def stream_news():
    return EventSourceResponse(news_orch.stream_events())


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
