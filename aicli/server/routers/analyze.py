"""FastAPI router for the UPSC Analyze pipeline."""
import shutil
import urllib.parse
from pathlib import Path
from typing import List

from fastapi import APIRouter, HTTPException, Depends, UploadFile, File, status
from fastapi.responses import FileResponse
from sse_starlette.sse import EventSourceResponse

from aicli.server.repositories.analyze_repository import AnalyzeRepository
from aicli.server.services.analyze_pipeline_service import AnalyzePipelineService
from aicli.server.dependencies import get_analyze_repository, get_analyze_service, ServerState
from aicli.server.schemas.analyze_schemas import (
    PDFListItemDTO,
    ProcessingStatusDTO,
    PageDTO,
    AnswerDTO,
    AnswerDimensionDTO,
    AggregationDTO,
    RunPipelineRequestDTO,
    ResetPipelineRequestDTO,
    RetryErrorsResponseDTO,
)
from aicli.server.orchestrator.base import BaseOrchestrator, SSEProgressContext

router = APIRouter()
analyze_orch = BaseOrchestrator()


# ── Read Endpoints ──────────────────────────────────────────────────

@router.get("/pdfs", response_model=List[PDFListItemDTO])
def list_pdfs(repo: AnalyzeRepository = Depends(get_analyze_repository)):
    return repo.get_pdf_list(ServerState.data_dir)


@router.get("/status", response_model=ProcessingStatusDTO)
def get_status(repo: AnalyzeRepository = Depends(get_analyze_repository)):
    return repo.get_status_metrics()


@router.get("/pdfs/{pdf_id}/pages", response_model=List[PageDTO])
def get_pdf_pages(pdf_id: int, repo: AnalyzeRepository = Depends(get_analyze_repository)):
    try:
        return repo.get_pdf_pages(pdf_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.get("/pdfs/{pdf_id}/answers", response_model=List[AnswerDTO])
def get_pdf_answers(pdf_id: int, repo: AnalyzeRepository = Depends(get_analyze_repository)):
    try:
        return repo.get_pdf_answers(pdf_id)
    except ValueError as e:
        raise HTTPException(status_code=404, detail=str(e))


@router.get("/answers/{answer_id}/dimensions", response_model=List[AnswerDimensionDTO])
def get_answer_dimensions(answer_id: int, repo: AnalyzeRepository = Depends(get_analyze_repository)):
    return repo.get_answer_dimensions(answer_id)


@router.get("/images/{pdf_name}/{image_name}")
def get_image(pdf_name: str, image_name: str):
    img_path = ServerState.cache_dir / pdf_name / image_name
    if not img_path.exists():
        raise HTTPException(status_code=404, detail="Image not found")
    return FileResponse(img_path)


@router.get("/aggregate", response_model=List[AggregationDTO])
def get_aggregate(repo: AnalyzeRepository = Depends(get_analyze_repository)):
    return repo.get_all_aggregations()


# ── Mutation Endpoints ──────────────────────────────────────────────

@router.post("/upload", status_code=status.HTTP_201_CREATED)
async def upload_pdfs(files: List[UploadFile] = File(...)):
    if not files:
        raise HTTPException(status_code=400, detail="No files uploaded")
    ServerState.data_dir.mkdir(parents=True, exist_ok=True)
    uploaded = [await _save_pdf(f) for f in files if f.filename.lower().endswith(".pdf")]
    return {"message": "Success", "files": uploaded}


@router.post("/reset")
def reset_pipeline(req: ResetPipelineRequestDTO, repo: AnalyzeRepository = Depends(get_analyze_repository)):
    repo.reset_pipeline(req.step)
    return {"ok": True, "reset_from_step": req.step}


@router.post("/retry-errors", response_model=RetryErrorsResponseDTO)
def retry_errors(repo: AnalyzeRepository = Depends(get_analyze_repository)):
    count = repo.retry_errors()
    return RetryErrorsResponseDTO(ok=True, cleared=count)


@router.delete("/pdfs/{pdf_file:path}")
def delete_pdf(pdf_file: str, repo: AnalyzeRepository = Depends(get_analyze_repository)):
    decoded = urllib.parse.unquote(pdf_file)
    repo.delete_pdf(decoded)
    _cleanup_pdf_files(decoded)
    return {"ok": True, "message": f"Deleted {decoded}"}


# ── Pipeline Execution ──────────────────────────────────────────────

@router.post("/run")
def run_pipeline(
    req: RunPipelineRequestDTO,
    service: AnalyzePipelineService = Depends(get_analyze_service),
):
    try:
        analyze_orch.dispatch(_analyze_worker, service=service, req=req)
        return {"ok": True, "message": "Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))


@router.get("/stream")
async def stream_progress():
    return EventSourceResponse(analyze_orch.stream_events())


@router.post("/stop")
def stop_pipeline():
    analyze_orch.abort()
    return {"ok": True, "message": "Abort requested"}


@router.get("/orchestrator-status")
def get_orchestrator_status():
    return {"is_running": analyze_orch.is_running}


# ── Private Helpers ─────────────────────────────────────────────────

def _analyze_worker(
    orch: BaseOrchestrator,
    service: AnalyzePipelineService,
    req: RunPipelineRequestDTO,
) -> None:
    """Background worker for the pipeline."""
    try:
        from aicli.server.orchestrator.base import PipelineAbortedError
        progress = SSEProgressContext(orch.queue, orch.abort_event)
        
        def log_cb(msg):
            if orch.abort_event.is_set():
                raise PipelineAbortedError("Pipeline aborted by user")
            orch.queue.put({"type": "log", "message": msg})

        service.run_full_pipeline(
            data_dir=ServerState.data_dir,
            cache_dir=ServerState.cache_dir,
            workers=req.workers,
            dpi=req.dpi,
            llm_model=req.llm_model,
            allow_reasoning=req.allow_reasoning,
            target_steps=req.target_steps,
            step_reasoning=req.step_reasoning,
            step_models=req.step_models,
            target_page_id=req.page_id,
            progress_callback=progress,
            log_callback=log_cb,
            abort_event=orch.abort_event,
        )
    finally:
        service._repo.close()


async def _save_pdf(file: UploadFile) -> str:
    """Save a single uploaded PDF to the data directory."""
    safe_name = Path(file.filename).name
    target_path = ServerState.data_dir / safe_name
    target_path.write_bytes(await file.read())
    return safe_name


def _cleanup_pdf_files(pdf_file: str) -> None:
    """Remove PDF file and its cache images from the filesystem."""
    image_dir = ServerState.cache_dir / Path(pdf_file).stem
    if image_dir.exists() and image_dir.is_dir():
        shutil.rmtree(image_dir)

    pdf_path = ServerState.data_dir / pdf_file
    if pdf_path.exists() and pdf_path.is_file():
        pdf_path.unlink()
