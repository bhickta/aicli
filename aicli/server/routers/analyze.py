import json
import urllib.parse
from pathlib import Path

from fastapi import APIRouter, HTTPException, BackgroundTasks, UploadFile, File
from fastapi.responses import FileResponse, JSONResponse
from pydantic import BaseModel
from sse_starlette.sse import EventSourceResponse

from aicli.domains.analyze.database import AnalyzeDB
from aicli.server.orchestrator.base import BaseOrchestrator, SSEProgressContext, ConsoleRedirect

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
        pdfs = conn.execute("SELECT pdf_file as filename, count(*) as page_count FROM pages GROUP BY pdf_file ORDER BY pdf_file").fetchall()
        # Mock an ID for Vue (just index + 1)
        db_pdfs = []
        for i, p in enumerate(pdfs):
            progress = db.get_pdf_progress(p["filename"])
            db_pdfs.append({
                "id": i+1, 
                "filename": p["filename"], 
                "page_count": p["page_count"],
                "progress": progress
            })
        
        # Also include uploaded but unprocessed PDFs from ServerState.data_dir
        processed_names = {p["filename"] for p in db_pdfs}
        if ServerState.data_dir.exists():
            idx = len(db_pdfs) + 1
            for child in ServerState.data_dir.glob("*.pdf"):
                if child.name not in processed_names:
                    db_pdfs.append({
                        "id": idx, 
                        "filename": child.name, 
                        "page_count": 0,
                        "progress": {"1": "pending", "2": "pending", "3": "pending", "4": "pending", "5": "pending"}
                    })
                    idx += 1
                    
        # Sort so we get consistent ordering
        db_pdfs.sort(key=lambda x: x["filename"])
        return db_pdfs

@router.post("/upload")
async def upload_pdfs(files: list[UploadFile] = File(...)):
    if not files:
        raise HTTPException(status_code=400, detail="No files uploaded")
    
    ServerState.data_dir.mkdir(parents=True, exist_ok=True)
    
    uploaded_files = []
    for file in files:
        if not file.filename.lower().endswith('.pdf'):
            continue
        
        safe_name = Path(file.filename).name
        target_path = ServerState.data_dir / safe_name
        
        # Write file in chunks to handle large PDFs without bursting RAM
        target_path.write_bytes(await file.read())
        uploaded_files.append(safe_name)
        
    return {"message": "Success", "files": uploaded_files}

@router.get("/pdfs/{pdf_id}/pages")
def get_pdf_pages(pdf_id: int):
    db = get_db()
    with db._get_conn() as conn:
        # Find the pdf name by index
        pdfs = conn.execute("SELECT DISTINCT pdf_file as filename FROM pages ORDER BY pdf_file").fetchall()
        if pdf_id < 1 or pdf_id > len(pdfs):
            raise HTTPException(404, "PDF not found")
        pdf_name = pdfs[pdf_id - 1]["filename"]
        
        pages = conn.execute(
            "SELECT id, page_number, pdf_file, image_path, transcription, classification "
            "FROM pages WHERE pdf_file = ? ORDER BY page_number",
            (pdf_name,)
        ).fetchall()
        
        results = []
        for p in pages:
            d = dict(p)
            results.append(d)
        return results

@router.get("/status")
def get_status():
    db = get_db()
    with db._get_conn() as conn:
        pdf_count = conn.execute("SELECT COUNT(DISTINCT pdf_file) FROM pages").fetchone()[0]
        pages_count = conn.execute("SELECT COUNT(*) FROM pages").fetchone()[0]
        classified = conn.execute("SELECT COUNT(*) FROM pages WHERE classification IS NOT NULL").fetchone()[0]
        
        # Count errors matching transcription prefixes
        errors = conn.execute(
            "SELECT COUNT(*) FROM pages WHERE transcription LIKE '[TRANSCRIPTION_ERROR%'"
        ).fetchone()[0]
        
        # Note: using `total_pdfs` to match frontend
        return {
            "total_pdfs": pdf_count,
            "total_pages": pages_count,
            "classified_pages": classified,
            "errors": {"OCR": errors} if errors else {}
        }

@router.get("/pdfs/{pdf_id}/answers")
def get_pdf_answers(pdf_id: int):
    db = get_db()
    with db._get_conn() as conn:
        pdfs = conn.execute("SELECT DISTINCT pdf_file as filename FROM pages ORDER BY pdf_file").fetchall()
        if pdf_id < 1 or pdf_id > len(pdfs):
            raise HTTPException(404, "PDF not found")
        pdf_name = pdfs[pdf_id - 1]["filename"]
        
        ans = conn.execute(
            "SELECT id, question_number, question_directive, question_text, raw_text, page_ids "
            "FROM answers WHERE pdf_file = ? ORDER BY CAST(question_number AS INTEGER)",
            (pdf_name,)
        ).fetchall()
        return [dict(a) for a in ans]

@router.get("/answers/{answer_id}/dimensions")
def get_answer_dimensions(answer_id: int):
    db = get_db()
    with db._get_conn() as conn:
        dims = conn.execute(
            "SELECT dimension_name, result_json FROM dimensions WHERE answer_id = ?",
            (answer_id,)
        ).fetchall()
        
        results = []
        for d in dims:
            row = dict(d)
            if row.get("result_json"):
                try:
                    row["result_json"] = json.loads(row["result_json"])
                except:
                    pass
            results.append(row)
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
        rows = conn.execute("SELECT dimension_name, aggregation_json, answer_count FROM dimension_aggregations").fetchall()
        
        results = {}
        for row in rows:
            dimension_name = row["dimension_name"]
            try:
                results[dimension_name] = {
                    "answer_count": row["answer_count"],
                    "aggregation_json": json.loads(row["aggregation_json"])
                }
            except:
                pass
        return results

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

@router.delete("/pdfs/{pdf_file:path}")
def delete_pdf(pdf_file: str):
    import shutil
    db = get_db()
    # Decode filename if encoded
    pdf_file = urllib.parse.unquote(pdf_file)
    
    # 1. Delete from database
    db.delete_pdf_data(pdf_file)
    
    # 2. Delete generated images
    pdf_stem = Path(pdf_file).stem
    image_dir = ServerState.cache_dir / pdf_stem
    if image_dir.exists() and image_dir.is_dir():
        shutil.rmtree(image_dir)
        
    # 3. Delete original PDF
    pdf_path = ServerState.data_dir / pdf_file
    if pdf_path.exists() and pdf_path.is_file():
        pdf_path.unlink()
        
    return {"ok": True, "message": f"Deleted {pdf_file}"}

# --- PIPELINE EXECUTION ---

class RunRequest(BaseModel):
    workers: int = 4
    dpi: int = 300
    llm_model: str = "gemma-4-26b-a4b"
    allow_reasoning: bool = True
    target_steps: list[int] | None = None
    step_reasoning: dict[int, bool] | None = None
    page_id: int | None = None

# Use a singleton instance of BaseOrchestrator for analyze (or instantiate per pipeline if wanted, but singleton limits concurrency nicely here)
analyze_orch = BaseOrchestrator()

def _analyze_worker(
    orch: BaseOrchestrator, 
    data_dir: Path, 
    workers: int, 
    dpi: int, 
    llm_model: str, 
    allow_reasoning: bool = True,
    target_steps: list[int] | None = None,
    step_reasoning: dict[int, bool] | None = None,
    target_page_id: int | None = None
):
    import aicli.server.pipelines.analyze as analyze_mod
    from aicli.server.pipelines.analyze import _get_db, _run_full_pipeline

    orig_make_progress = getattr(analyze_mod, "_make_progress", None)
    orig_console = getattr(analyze_mod, "console", None)
    orig_print_success = getattr(analyze_mod, "print_success", None)
    orig_print_error = getattr(analyze_mod, "print_error", None)

    try:
        db = _get_db(data_dir)
        
        analyze_mod._make_progress = lambda: SSEProgressContext(orch.queue)
        analyze_mod.console = ConsoleRedirect(orch.queue)
        analyze_mod.print_success = lambda msg: orch.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
        analyze_mod.print_error = lambda msg, exc: orch.queue.put({"type": "log", "message": f"[ERROR] {msg} - {exc}"})
        
        _run_full_pipeline(
            data_dir=data_dir,
            db=db,
            workers=workers,
            dpi=dpi,
            pdf_files=None,
            llm_model=llm_model,
            allow_reasoning=allow_reasoning,
            target_steps=target_steps,
            step_reasoning=step_reasoning,
            target_page_id=target_page_id
        )
        db.close()
    finally:
        # Restore originals
        if orig_make_progress: analyze_mod._make_progress = orig_make_progress
        if orig_console: analyze_mod.console = orig_console
        if orig_print_success: analyze_mod.print_success = orig_print_success
        if orig_print_error: analyze_mod.print_error = orig_print_error

@router.post("/run")
def run_pipeline(req: RunRequest):
    try:
        analyze_orch.dispatch(
            _analyze_worker,
            data_dir=ServerState.data_dir,
            workers=req.workers,
            dpi=req.dpi,
            llm_model=req.llm_model,
            allow_reasoning=req.allow_reasoning,
            target_steps=req.target_steps,
            step_reasoning=req.step_reasoning,
            target_page_id=req.page_id,
        )
        return {"ok": True, "message": "Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

@router.get("/stream")
async def stream_progress():
    return EventSourceResponse(analyze_orch.stream_events())
