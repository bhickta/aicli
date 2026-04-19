import urllib.parse
from pathlib import Path

from fastapi import APIRouter, HTTPException
from fastapi.responses import EventSourceResponse
from pydantic import BaseModel

from aicli.server.orchestrator.base import BaseOrchestrator, SSEProgressContext, ConsoleRedirect

router = APIRouter()

# Share ServerState from analyze (or define globally)
from aicli.server.routers.analyze import ServerState

video_course_orch = BaseOrchestrator()

class VideoCourseRequest(BaseModel):
    target_dir: str
    whisper_model: str = "large-v3"
    cleanup: str = "keep"
    w1: int = 2
    w2: int = 12
    w3: int = 12
    llm_model: str = "gemma-4-26b-a4b"

def _video_course_worker(orch: BaseOrchestrator, data_dir: Path, req: VideoCourseRequest):
    import aicli.server.pipelines.video_course as video_mod
    
    orig_console = getattr(video_mod, "console", None)
    try:
        # Monkey patch core console and print methods
        video_mod.console = ConsoleRedirect(orch.queue)
        # Note: video_course manually invokes rich Progress. We will need to monkey patch rich.progress.Progress too if we want native progress bars, OR we can let console redirect catch the raw text.
        # For full progress bar support, we'd mock rich.progress like AnalyzeOrchestrator did using _make_progress. In video_course, it uses contextual 'with Progress(...) as progress'.
        
        video_mod.process_course(
            target_dir=data_dir / req.target_dir,
            whisper_model=req.whisper_model,
            cleanup=req.cleanup,
            w1=req.w1,
            w2=req.w2,
            w3=req.w3,
            llm_model=req.llm_model,
            max_merge_hours=0.0,
            notes_llm=req.llm_model
        )
    finally:
        if orig_console: video_mod.console = orig_console

@router.post("/course/run")
def run_video_course(req: VideoCourseRequest):
    try:
        video_course_orch.dispatch(
            _video_course_worker,
            data_dir=ServerState.data_dir,
            req=req
        )
        return {"ok": True, "message": "Video Course Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

@router.get("/course/stream")
async def stream_video_course():
    return EventSourceResponse(video_course_orch.stream_events())
