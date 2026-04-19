import asyncio
import json
from pathlib import Path
from typing import Any

from fastapi import FastAPI, Request
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import HTMLResponse, FileResponse
from fastapi.staticfiles import StaticFiles

from aicli.server.routers.analyze import router as analyze_router

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

# Optionally host the built Vue UI in production mode
# app.mount("/", StaticFiles(directory="built-ui-dir", html=True), name="ui")

@app.get("/api/health")
def health_check():
    return {"status": "ok"}
