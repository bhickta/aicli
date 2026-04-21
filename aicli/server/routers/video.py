"""FastAPI router for Video pipelines (Course, Compress, Tag, Notes)."""
from pathlib import Path

from fastapi import APIRouter, HTTPException
from sse_starlette.sse import EventSourceResponse

from aicli.server.orchestrator.base import BaseOrchestrator
from aicli.server.orchestrator.console_patcher import ConsolePatcher
from aicli.server.dependencies import ServerState
from aicli.server.schemas.video_schemas import (
    VideoCourseRequestDTO,
    VideoCompressRequestDTO,
    VideoTagRequestDTO,
    VideoNotesRequestDTO,
)

router = APIRouter()
video_orch = BaseOrchestrator()


# ── Workers ─────────────────────────────────────────────────────────

def _video_course_worker(orch: BaseOrchestrator, data_dir: Path, req: VideoCourseRequestDTO) -> None:
    import aicli.server.pipelines.video_course as video_mod

    with ConsolePatcher(video_mod, orch.queue):
        video_mod.process_course(
            target_dir=data_dir / req.target_dir,
            whisper_model=req.whisper_model,
            cleanup=req.cleanup,
            w1=req.w1, w2=req.w2, w3=req.w3,
            llm_model=req.llm_model,
            llm_thinking=req.llm_thinking,
            max_merge_hours=req.max_merge_hours,
            notes_llm=req.llm_model,
        )


def _video_compress_worker(orch: BaseOrchestrator, req: VideoCompressRequestDTO) -> None:
    import aicli.server.pipelines.video_compress as compress_mod

    target = _resolve_path(req.target_path)
    with ConsolePatcher(compress_mod, orch.queue):
        compress_mod.compress_video(
            target_path=target,
            resolution=req.resolution,
            preset=req.preset,
            overwrite=req.overwrite,
            workers=req.workers,
            crf=req.crf,
            fps=req.fps,
            fast_skip=req.fast_skip,
        )


def _video_tag_worker(orch: BaseOrchestrator, req: VideoTagRequestDTO) -> None:
    import aicli.server.pipelines.video_tag as tag_mod

    target = _resolve_path(req.target_path)
    with ConsolePatcher(tag_mod, orch.queue):
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
            whisper_model=req.whisper_model,
        )


def _video_notes_worker(orch: BaseOrchestrator, req: VideoNotesRequestDTO) -> None:
    import aicli.server.pipelines.video_notes as notes_mod

    target = _resolve_path(req.target_path)
    with ConsolePatcher(notes_mod, orch.queue):
        notes_mod.process_notes(
            target_path=target,
            overwrite=req.overwrite,
            style=req.style,
        )


# ── Endpoints ───────────────────────────────────────────────────────

@router.post("/course/run")
def run_video_course(req: VideoCourseRequestDTO):
    return _dispatch(video_orch, _video_course_worker, "Video Course Pipeline", data_dir=ServerState.data_dir, req=req)


@router.post("/compress/run")
def run_video_compress(req: VideoCompressRequestDTO):
    return _dispatch(video_orch, _video_compress_worker, "Video Compress Pipeline", req=req)


@router.post("/tag/run")
def run_video_tag(req: VideoTagRequestDTO):
    return _dispatch(video_orch, _video_tag_worker, "Video Tag Pipeline", req=req)


@router.post("/notes/run")
def run_video_notes(req: VideoNotesRequestDTO):
    return _dispatch(video_orch, _video_notes_worker, "Video Notes Pipeline", req=req)


@router.get("/course/stream")
async def stream_video_course():
    return EventSourceResponse(video_orch.stream_events())


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
