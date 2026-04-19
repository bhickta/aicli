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

class VideoCompressRequest(BaseModel):
    target_path: str
    resolution: int = 240
    preset: str = "light"
    overwrite: bool = False
    workers: int = 4
    crf: int = None
    fps: str = None
    fast_skip: bool = False

def _video_compress_worker(orch: BaseOrchestrator, req: VideoCompressRequest):
    import aicli.server.pipelines.video_compress as compress_mod
    orig_console = getattr(compress_mod, "console", None)
    orig_print_success = getattr(compress_mod, "print_success", None)
    orig_print_error = getattr(compress_mod, "print_error", None)
    try:
        compress_mod.console = ConsoleRedirect(orch.queue)
        compress_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        compress_mod.print_error = lambda msg, exc=None: orch.queue.put({"type": "log", "message": f"[ERROR] {msg} {exc or ''}"})
        
        target = Path(req.target_path)
        if not target.is_absolute():
            target = ServerState.data_dir / req.target_path

        compress_mod.compress_video(
            target_path=target,
            resolution=req.resolution,
            preset=req.preset,
            overwrite=req.overwrite,
            workers=req.workers,
            crf=req.crf,
            fps=req.fps,
            fast_skip=req.fast_skip
        )
    finally:
        if orig_console: compress_mod.console = orig_console
        if orig_print_success: compress_mod.print_success = orig_print_success
        if orig_print_error: compress_mod.print_error = orig_print_error

@router.post("/compress/run")
def run_video_compress(req: VideoCompressRequest):
    try:
        # Re-use the existing orchestrator
        video_course_orch.dispatch(_video_compress_worker, req=req)
        return {"ok": True, "message": "Video Compress Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

class VideoTagRequest(BaseModel):
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

def _video_tag_worker(orch: BaseOrchestrator, req: VideoTagRequest):
    import aicli.server.pipelines.video_tag as tag_mod
    orig_console = getattr(tag_mod, "console", None)
    orig_print_success = getattr(tag_mod, "print_success", None)
    orig_print_error = getattr(tag_mod, "print_error", None)
    try:
        tag_mod.console = ConsoleRedirect(orch.queue)
        tag_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        tag_mod.print_error = lambda msg, exc=None: orch.queue.put({"type": "log", "message": f"[ERROR] {msg} {exc or ''}"})
        
        target = Path(req.target_path)
        if not target.is_absolute():
            target = ServerState.data_dir / req.target_path

        tag_mod.tag_video(
            target_path=target,
            write=req.write,
            no_rename=req.no_rename,
            full_cc=req.full_cc,
            text_thumb=req.text_thumb,
            retranscribe=req.retranscribe,
            transcribe_only=req.transcribe_only,
            workers=req.workers,
            clip_every=req.clip_every,
            clip_len=req.clip_len,
            save_txt=req.save_txt,
            whisper_model=req.whisper_model
        )
    finally:
        if orig_console: tag_mod.console = orig_console
        if orig_print_success: tag_mod.print_success = orig_print_success
        if orig_print_error: tag_mod.print_error = orig_print_error

@router.post("/tag/run")
def run_video_tag(req: VideoTagRequest):
    try:
        video_course_orch.dispatch(_video_tag_worker, req=req)
        return {"ok": True, "message": "Video Tag Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


class VideoNotesRequest(BaseModel):
    target_path: str
    overwrite: bool = False
    style: str = "bullet"

def _video_notes_worker(orch: BaseOrchestrator, req: VideoNotesRequest):
    import aicli.server.pipelines.video_notes as notes_mod
    orig_console = getattr(notes_mod, "console", None)
    try:
        notes_mod.console = ConsoleRedirect(orch.queue)
        
        target = Path(req.target_path)
        if not target.is_absolute():
            target = ServerState.data_dir / req.target_path

        notes_mod.process_notes(
            target_path=target,
            overwrite=req.overwrite,
            style=req.style
        )
    finally:
        if orig_console: notes_mod.console = orig_console

@router.post("/notes/run")
def run_video_notes(req: VideoNotesRequest):
    try:
        video_course_orch.dispatch(_video_notes_worker, req=req)
        return {"ok": True, "message": "Video Notes Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))
