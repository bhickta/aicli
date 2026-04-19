import urllib.parse
from pathlib import Path

from fastapi import APIRouter, HTTPException
from fastapi.responses import EventSourceResponse
from pydantic import BaseModel

from aicli.server.orchestrator.base import BaseOrchestrator, SSEProgressContext, ConsoleRedirect

router = APIRouter()

# Share ServerState from analyze (or define globally)
from aicli.server.routers.analyze import ServerState

news_orch = BaseOrchestrator()

class NewsProcessRequest(BaseModel):
    json_path: str
    output: str = None
    workers: int = 4
    threshold: float = 0.8
    force_merge: bool = False
    no_cache: bool = False


def _news_process_worker(orch: BaseOrchestrator, req: NewsProcessRequest):
    import aicli.server.pipelines.news as news_mod
    
    orig_console = getattr(news_mod, "console", None)
    orig_print_success = getattr(news_mod, "print_success", None)
    orig_print_error = getattr(news_mod, "print_error", None)
    try:
        news_mod.console = ConsoleRedirect(orch.queue)
        news_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        news_mod.print_error = lambda msg, exc=None: orch.queue.put({"type": "log", "message": f"[ERROR] {msg} {exc or ''}"})
        
        json_file = Path(req.json_path)
        if not json_file.is_absolute():
            json_file = ServerState.data_dir / req.json_path
            
        out_file = None
        if req.output:
            out_file = Path(req.output)
            if not out_file.is_absolute():
                out_file = ServerState.data_dir / req.output

        news_mod.process_news(
            json_path=json_file,
            output=out_file,
            workers=req.workers,
            threshold=req.threshold,
            force_merge=req.force_merge,
            no_cache=req.no_cache
        )
    finally:
        if orig_console: news_mod.console = orig_console
        if orig_print_success: news_mod.print_success = orig_print_success
        if orig_print_error: news_mod.print_error = orig_print_error

@router.post("/process")
def run_news_process(req: NewsProcessRequest):
    try:
        news_orch.dispatch(_news_process_worker, req=req)
        return {"ok": True, "message": "News God-Mode Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


class NewsDedupeRequest(BaseModel):
    file_path: str
    output: str = None
    workers: int = 10
    threshold: float = 0.8

def _news_dedupe_worker(orch: BaseOrchestrator, req: NewsDedupeRequest):
    import aicli.server.pipelines.news as news_mod
    orig_console = getattr(news_mod, "console", None)
    orig_print_success = getattr(news_mod, "print_success", None)
    orig_print_error = getattr(news_mod, "print_error", None)
    try:
        news_mod.console = ConsoleRedirect(orch.queue)
        news_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        news_mod.print_error = lambda msg, exc=None: orch.queue.put({"type": "log", "message": f"[ERROR] {msg} {exc or ''}"})
        
        excel_file = Path(req.file_path)
        if not excel_file.is_absolute():
            excel_file = ServerState.data_dir / req.file_path
            
        out_file = None
        if req.output:
            out_file = Path(req.output)
            if not out_file.is_absolute():
                out_file = ServerState.data_dir / req.output

        news_mod.dedupe(
            file_path=excel_file,
            output=out_file,
            threshold=req.threshold,
            workers=req.workers
        )
    finally:
        if orig_console: news_mod.console = orig_console
        if orig_print_success: news_mod.print_success = orig_print_success
        if orig_print_error: news_mod.print_error = orig_print_error

@router.post("/dedupe")
def run_news_dedupe(req: NewsDedupeRequest):
    try:
        news_orch.dispatch(_news_dedupe_worker, req=req)
        return {"ok": True, "message": "News Dedupe Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

@router.get("/stream")
async def stream_news():
    return EventSourceResponse(news_orch.stream_events())
