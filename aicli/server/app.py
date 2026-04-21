import asyncio
import json
import os
from pathlib import Path
from typing import Any

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import HTMLResponse, FileResponse
from fastapi.staticfiles import StaticFiles

from aicli.server.routers.analyze import router as analyze_router
from aicli.server.routers.video import router as video_router
from aicli.server.routers.news import router as news_router
from aicli.server.routers.image import router as image_router
from aicli.server.routers.settings import router as settings_router
from aicli.server.routers.fs import router as fs_router
from fastapi import HTTPException
import logging

# Silence high-frequency log noise from status polling
class EndpointFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        # Suppress any log lines containing these high-frequency paths
        msg = record.getMessage()
        return "/api/analyze/status" not in msg and "/api/health" not in msg and "/api/video/course/stream" not in msg

logging.getLogger("uvicorn.access").addFilter(EndpointFilter())

app = FastAPI(title="AICLI Unified Control Center", description="Web API for all AICLI features")

# Enable CORS for the Vue dev server
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Register routers for diff sub-systems
app.include_router(analyze_router, prefix="/api/analyze", tags=["Analyze"])
app.include_router(video_router, prefix="/api/video", tags=["Video"])
app.include_router(news_router, prefix="/api/news", tags=["News"])
app.include_router(image_router, prefix="/api/image", tags=["Image"])
app.include_router(settings_router, prefix="/api/settings", tags=["Settings"])
app.include_router(fs_router, prefix="/api/fs", tags=["FS"])

@app.get("/api/health")
def health_check():
    return {"status": "ok"}

# Frontend dist folder relative to this file
BASE_DIR = Path(__file__).parent.parent.parent
DIST_DIR = BASE_DIR / "frontend" / "dist"

from fastapi.responses import HTMLResponse, FileResponse, RedirectResponse

if os.environ.get("AICLI_DEV_MODE") == "1":
    @app.get("/{full_path:path}")
    async def redirect_to_vite(full_path: str):
        if full_path.startswith("api/"):
            raise HTTPException(status_code=404, detail="API route not found")
        return RedirectResponse(f"http://localhost:5173/{full_path}")
elif DIST_DIR.exists():
    app.mount("/assets", StaticFiles(directory=str(DIST_DIR / "assets")), name="assets")
    
    @app.get("/{full_path:path}")
    async def serve_spa(full_path: str):
        # Prevent shadowing API routes
        if full_path.startswith("api/"):
            raise HTTPException(status_code=404, detail="API route not found")
        # Check if the requested file exists in dist (e.g., manifest.json, favicon.ico)
        req_path = DIST_DIR / full_path
        if req_path.exists() and req_path.is_file():
            return FileResponse(req_path)
        # Fallback to index.html for SPA client-side routing
        return FileResponse(DIST_DIR / "index.html")
else:
    print(f"Warning: Frontend dist directory {DIST_DIR} not found. UI will not be available.")
