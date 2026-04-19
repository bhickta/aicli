import json
import urllib.parse
from pathlib import Path

from fastapi import APIRouter, HTTPException, BackgroundTasks
from fastapi.responses import FileResponse, JSONResponse
from pydantic import BaseModel
from sse_starlette.sse import EventSourceResponse

from aicli.domains.analyze.database import AnalyzeDB
from aicli.server.orchestrator import AnalyzeOrchestrator

router = APIRouter()

# Dependency injection for DB not fully needed yet if we assume local directory
# But aicli usually defaults to the current executing directory or passed via CLI
# We will use a global configuration for the server session.
class ServerState:
    data_dir = Path(".")
    cache_dir = Path(".")

# We need a way to set these from the main app.
# For now, let's inject them explicitly.

def get_db():
    try:
        db = AnalyzeDB(ServerState.data_dir / "analyze.db")
        return db
    except FileNotFoundError:
        raise HTTPException(status_code=404, detail="Database analyze.db not found. Run analysis first.")

@router.get("/pdfs")
def list_pdfs():
    db = get_db()
    with db._get_conn() as conn:
        pdfs = conn.execute("SELECT id, filename, page_count FROM pdfs ORDER BY filename").fetchall()
        return [dict(p) for p in pdfs]

@router.get("/pdfs/{pdf_id}/pages")
def get_pdf_pages(pdf_id: int):
    db = get_db()
    with db._get_conn() as conn:
        pages = conn.execute(
            "SELECT id, page_number, image_path, transcription, classification, dimensions, answers "
            "FROM pages WHERE pdf_id = ? ORDER BY page_number",
            (pdf_id,)
        ).fetchall()
        
        results = []
        for p in pages:
            d = dict(p)
            for json_field in ['dimensions', 'answers']:
                if d.get(json_field):
                    try:
                        d[json_field] = json.loads(d[json_field])
                    except:
                        pass
            results.append(d)
        return results

@router.get("/images/{pdf_name}/{image_name}")
def get_image(pdf_name: str, image_name: str):
    pdf_name_dec = urllib.parse.unquote(pdf_name)
    image_name_dec = urllib.parse.unquote(image_name)
    img_path = ServerState.cache_dir / pdf_name_dec / image_name_dec
    if not img_path.exists():
        raise HTTPException(status_code=404, detail="Image not found")
    return FileResponse(img_path)

@router.get("/aggregate")
def get_aggregate():
    db = get_db()
    with db._get_conn() as conn:
        agg = conn.execute("SELECT output FROM aggregation ORDER BY created_at DESC LIMIT 1").fetchone()
        if agg and agg["output"]:
            try:
                return json.loads(agg["output"])
            except:
                pass
    return None

class ResetRequest(BaseModel):
    step: int = 2

@router.post("/reset")
def reset_pipeline(req: ResetRequest):
    db = get_db()
    db.reset_from_step(req.step)
    return {"ok": True, "reset_from_step": req.step}

@router.post("/retry-errors")
def retry_errors():
    db = get_db()
    with db._get_conn() as conn:
        cur = conn.execute(
            "UPDATE pages SET transcription = NULL, processed = 0 "
            "WHERE transcription LIKE '[TRANSCRIPTION_ERROR%'"
        )
        conn.commit()
    return {"ok": True, "cleared": cur.rowcount}

# --- PIPELINE EXECUTION ---

class RunRequest(BaseModel):
    workers: int = 4
    dpi: int = 200
    llm_model: str = "gemma-4-26b-a4b"

@router.post("/run")
def run_pipeline(req: RunRequest):
    orch = AnalyzeOrchestrator.get_instance()
    try:
        orch.run_pipeline(
            data_dir=ServerState.data_dir,
            workers=req.workers,
            dpi=req.dpi,
            llm_model=req.llm_model
        )
        return {"ok": True, "message": "Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

@router.get("/stream")
async def stream_progress():
    orch = AnalyzeOrchestrator.get_instance()
    return EventSourceResponse(orch.stream_events())
