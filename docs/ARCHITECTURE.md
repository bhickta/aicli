# AICLI — Architecture Reference

> **One-line summary:** A Python (FastAPI + Typer) backend with a Vue 3 (Vite) frontend that orchestrates local AI pipelines (LM Studio, Whisper, FFmpeg) for PDF analysis, video processing, news clustering, and image management.

---

## Technology Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| **Runtime** | Python | ≥ 3.10 |
| **CLI Framework** | Typer + Rich | ≥ 0.12 |
| **Web Framework** | FastAPI + Uvicorn | ≥ 0.110 |
| **AI Provider** | OpenAI SDK → LM Studio (local) | ≥ 1.14 |
| **Transcription** | faster-whisper | ≥ 1.0 |
| **Embeddings** | sentence-transformers | ≥ 3.4 |
| **Database** | SQLite (WAL mode, thread-safe) | stdlib |
| **PDF Rendering** | pdf2image (poppler) | ≥ 1.16 |
| **Video** | FFmpeg / FFprobe (subprocess) | system |
| **Frontend** | Vue 3 (Composition API) + Vite 5 | ≥ 3.5 |
| **Package Manager** | uv (Python), npm (JS) | — |
| **Build** | hatchling | — |

---

## Top-Level Directory Structure

```
aicli/                          ← Project root
├── aicli/                      ← Python package (the app)
│   ├── main.py                 ← CLI entry point (Typer app)
│   ├── config.py               ← App config: LM Studio URL, model resolution
│   ├── cli/commands/            ← CLI subcommands (server.py)
│   ├── core/interfaces.py       ← Abstract base: ImageVisionProvider
│   ├── providers/lm_studio.py   ← LM Studio OpenAI-compat provider
│   ├── domains/                 ← Domain models & DB (analyze, news)
│   ├── services/                ← Business logic services
│   └── server/                  ← FastAPI web layer
│       ├── app.py               ← FastAPI app factory + router registration
│       ├── dependencies.py      ← DI wiring (repos, services)
│       ├── routers/             ← HTTP boundary (5 routers)
│       ├── schemas/             ← Pydantic request/response DTOs
│       ├── services/            ← Server-specific orchestration services
│       ├── repositories/        ← Data access layer (DB, Excel, cache)
│       ├── orchestrator/        ← Background thread + SSE streaming
│       ├── pipelines/           ← Pipeline entry points (thin wrappers)
│       ├── constants/           ← Typed enums and config constants
│       └── tasks/               ← (empty — reserved for future async tasks)
├── frontend/                   ← Vue 3 SPA
│   ├── src/
│   │   ├── App.vue              ← Root: global sidebar + workspace switcher
│   │   ├── api/                 ← HTTP clients (class-based)
│   │   ├── components/          ← Vue components by workspace
│   │   ├── composables/         ← Shared reactive hooks
│   │   ├── constants/           ← API_BASE, pipeline step defs
│   │   ├── styles/              ← Design tokens + component CSS
│   │   └── utils/               ← Pure formatting helpers
│   ├── package.json
│   └── vite.config.js           ← Dev proxy: /api → localhost:8765
├── data/                       ← Runtime data directory (PDFs, DBs)
├── .agents/                    ← AI agent instruction files
├── pyproject.toml              ← Python project config
└── uv.lock                    ← Lockfile
```

---

## Architectural Layers

```
┌──────────────────────────────────────────────────┐
│                   Vue 3 Frontend                 │
│  App.vue → Workspaces → Components → Composables │
│                    ↕ HTTP / SSE                   │
├──────────────────────────────────────────────────┤
│              FastAPI Router Layer                 │
│  routers/{analyze,video,news,image,settings}.py  │
│                    ↓                              │
├──────────────────────────────────────────────────┤
│          Server Services / Orchestrator           │
│  AnalyzePipelineService, BaseOrchestrator, etc.  │
│                    ↓                              │
├──────────────────────────────────────────────────┤
│           Domain Services (Business Logic)        │
│  services/analyze/*, services/video/*, etc.      │
│                    ↓                              │
├──────────────────────────────────────────────────┤
│            Repositories (Data Access)             │
│  AnalyzeRepository, NewsExcelRepository, etc.    │
│                    ↓                              │
├──────────────────────────────────────────────────┤
│          Domain Layer (DB, Models, Config)         │
│  domains/analyze/db/*, domains/analyze/prompts   │
│                    ↓                              │
├──────────────────────────────────────────────────┤
│         Providers (External Integrations)          │
│  LMStudioProvider (OpenAI SDK), Whisper, FFmpeg   │
└──────────────────────────────────────────────────┘
```

### Rules

1. **Routers** handle HTTP only — no business logic
2. **Server Services** orchestrate domain services and manage pipeline execution
3. **Domain Services** (`services/analyze/*`) contain pure business logic
4. **Repositories** handle all data access (SQLite, Excel, filesystem cache)
5. **Providers** abstract external AI/tool integrations behind interfaces
6. **Components** never call `fetch` directly — they use API clients and composables

---

## The 5 Workspaces

### 1. Analyze (UPSC PDF Pipeline)

The most complex feature. A 7-step pipeline that processes UPSC exam answer sheets.

| Step | Name | Service | What it does |
|------|------|---------|-------------|
| 1 | PDF → Images | `PDFConverterService` | Converts PDF pages to PNG images via poppler |
| 2 | OCR Transcribe | `AnswerTranscriberService` | Vision model transcribes handwritten text |
| 3 | Page Classify | `PageClassifierService` | Classifies pages: cover/answer/continuation/blank |
| 4 | Answer Segment | `AnswerSegmenterService` | Groups pages into individual question-answers |
| 5 | Dimension Analyze | `DimensionAnalyzerService` | Analyzes each answer across configurable dimensions |
| 6 | Aggregation | `AggregationService` | Cross-PDF pattern synthesis |
| 7 | Report | `ReportGeneratorService` | Generates markdown report |

**Key files:**
- Router: `server/routers/analyze.py`
- Pipeline service: `server/services/analyze_pipeline_service.py`
- Domain DB: `domains/analyze/db/` (mixins: `base.py`, `pages.py`, `answers.py`, `dimensions.py`, `logs.py`)
- Prompts: `domains/analyze/prompts.yaml`
- Constants: `server/constants/analyze_constants.py`
- Repository: `server/repositories/analyze_repository.py`

### 2. Video (Course Builder)

Four sub-pipelines for video processing:

| Pipeline | Endpoint | What it does |
|----------|----------|-------------|
| Course | `/api/video/course/run` | Transcribe → Tag → Compress → Merge → Notes |
| Compress | `/api/video/compress/run` | GPU-accelerated NVENC compression |
| Tag | `/api/video/tag/run` | AI-powered metadata tagging + renaming |
| Notes | `/api/video/notes/run` | Generate study notes from transcripts |

**Key files:**
- Router: `server/routers/video.py`
- Pipeline modules: `server/pipelines/video_*.py`
- Services: `services/video/*` (ffmpeg, ffprobe, transcriber, tagger, notes, compress, merge)

### 3. News (Current Affairs)

| Pipeline | Endpoint | What it does |
|----------|----------|-------------|
| Process | `/api/news/process` | JSON → clustered + deduplicated Excel |
| Dedupe | `/api/news/dedupe` | Semantic dedup of existing Excel |

**Key files:**
- Router: `server/routers/news.py`
- Repository: `server/repositories/news_excel_repository.py`
- Services: `server/services/news_clustering_service.py`, `news_reasoning_service.py`

### 4. Image (Renaming & Cleanup)

| Pipeline | Endpoint | What it does |
|----------|----------|-------------|
| Rename | `/api/image/rename` | AI-powered image renaming |
| Clean | `/api/image/clean` | Junk image detection + cleanup |
| Digitize | `/api/image/digitize` | Markdown-embedded image digitization |

**Key files:**
- Router: `server/routers/image.py`
- Service: `services/image_renamer.py`

### 5. Settings

Simple config read/write for LM Studio connection details.

- Router: `server/routers/settings.py`
- Config: `config.py` (persisted to `~/.config/aicli/settings.json`)

---

## API Route Map

| Method | Path | Router | Description |
|--------|------|--------|-------------|
| `GET` | `/api/health` | app.py | Health check |
| `GET` | `/api/analyze/pdfs` | analyze | List all PDFs |
| `GET` | `/api/analyze/status` | analyze | Pipeline metrics |
| `GET` | `/api/analyze/pdfs/{id}/pages` | analyze | Pages for a PDF |
| `GET` | `/api/analyze/pdfs/{id}/answers` | analyze | Answers for a PDF |
| `GET` | `/api/analyze/answers/{id}/dimensions` | analyze | Dimension results |
| `GET` | `/api/analyze/images/{pdf}/{img}` | analyze | Serve cached images |
| `GET` | `/api/analyze/aggregate` | analyze | Cross-PDF aggregations |
| `POST` | `/api/analyze/upload` | analyze | Upload PDFs |
| `POST` | `/api/analyze/run` | analyze | Start pipeline |
| `POST` | `/api/analyze/reset` | analyze | Reset from step N |
| `POST` | `/api/analyze/retry-errors` | analyze | Clear transcription errors |
| `DELETE` | `/api/analyze/pdfs/{file}` | analyze | Delete PDF + data |
| `GET` | `/api/analyze/stream` | analyze | SSE progress stream |
| `POST` | `/api/video/course/run` | video | Start course pipeline |
| `POST` | `/api/video/compress/run` | video | Start compress |
| `POST` | `/api/video/tag/run` | video | Start tagging |
| `POST` | `/api/video/notes/run` | video | Start notes |
| `GET` | `/api/video/course/stream` | video | SSE progress stream |
| `POST` | `/api/news/process` | news | Process news feed |
| `POST` | `/api/news/dedupe` | news | Deduplicate Excel |
| `GET` | `/api/news/stream` | news | SSE progress stream |
| `POST` | `/api/image/rename` | image | Rename images |
| `POST` | `/api/image/clean` | image | Clean junk images |
| `POST` | `/api/image/digitize` | image | Digitize images |
| `GET` | `/api/image/stream` | image | SSE progress stream |
| `GET` | `/api/settings` | settings | Get config |
| `POST` | `/api/settings` | settings | Update config |

---

## Background Pipeline Execution (SSE Orchestrator)

All long-running pipelines use the same pattern:

```
1. Frontend POSTs to /api/{domain}/run
2. Router calls orchestrator.dispatch(worker_fn, **kwargs)
3. Orchestrator spawns a daemon thread running the worker
4. Worker pushes events to a thread-safe queue
5. Frontend connects to /api/{domain}/stream (SSE)
6. Orchestrator yields queued events as server-sent events
```

**Event types:**
- `{"type": "status", "status": "started|completed|error"}`
- `{"type": "task_add", "task_id": N, "description": "...", "total": N}`
- `{"type": "task_progress", "task_id": N, "completed": N, "total": N}`
- `{"type": "log", "message": "..."}`

**Key files:**
- `server/orchestrator/base.py` → `BaseOrchestrator`, `SSEProgressContext`, `ConsoleRedirect`
- `server/orchestrator/console_patcher.py` → `ConsolePatcher` (monkey-patches Rich console to SSE)

---

## Frontend Architecture

### Component Hierarchy

```
App.vue (workspace switcher)
├── AnalyzeStudio.vue          ← 3-panel layout
│   ├── AnalyzeSidebar.vue     ← PDF list + upload
│   ├── Content area (pages/answers/aggregations)
│   │   ├── PageInspector.vue  ← Page detail viewer
│   │   └── AnswerList.vue     ← Segmented answers
│   ├── PipelineRunner.vue     ← Config + execution + logs
│   └── AnalyzeBanner.vue      ← Status bar
│
├── VideoStudio.vue            ← Tab-based sub-tools
│   ├── VideoSidebar.vue
│   ├── CourseBuilder.vue
│   ├── GPUCompress.vue
│   ├── AITagging.vue
│   ├── StudyNotes.vue
│   └── LiveTerminal.vue
│
├── NewsStudio.vue
│   ├── NewsSidebar.vue
│   ├── ProcessFeed.vue
│   └── ExcelDedupe.vue
│
├── ImageStudio.vue
│   ├── ImageSidebar.vue
│   ├── AIRenamer.vue
│   ├── JunkCleaner.vue
│   └── MarkdownDigitize.vue
│
└── SettingsStudio.vue
```

### Key Composables

| Composable | Purpose |
|-----------|---------|
| `useAnalyzePipeline.ts` | Analyze-specific pipeline state, SSE, log parsing |
| `useStreamPipeline.ts` | Generic pipeline runner (video, news, image) |
| `useAnalyzeStatus.ts` | Periodic status polling |
| `usePageInspector.ts` | Page detail fetching |
| `useImagePipeline.ts` | Image pipeline wrapper |
| `useNewsPipeline.ts` | News pipeline wrapper |
| `useVideoPipeline.ts` | Video pipeline wrapper |

### Design System

- **Tokens:** `styles/tokens.css` — CSS custom properties for colors, spacing, radii
- **Theme:** Dark mode by default (`--bg-primary: #0a0a0f`)
- **Font:** Inter (Google Fonts)
- **Component CSS:** Separate files in `styles/` (sidebar, runner, inspector, etc.)

---

## Configuration

### LM Studio Connection

```json
// ~/.config/aicli/settings.json
{
  "lm_studio_base_url": "http://localhost:1234/v1",
  "lm_studio_api_key": "sk-...",
  "model_name": "local-model"
}
```

### Dynamic Model Resolution

`config.py:resolve_dynamic_model()` queries LM Studio's native API (`/api/v1/models`) to:
1. Find already-loaded models (prefer those)
2. Auto-load the first available model if none are loaded
3. Apply optimized settings for large MoE models (26B+)

### Analysis Prompts

All LLM prompts live in `domains/analyze/prompts.yaml` — editable without code changes:
- `transcription.prompt` — OCR instructions
- `classification.prompt` — Page type classification
- `segmentation.prompt` — Answer boundary detection
- `metadata.prompt` — Candidate info extraction
- `dimensions.*` — Per-dimension analysis prompts
- `aggregation.prompt` — Cross-PDF synthesis

---

## How to Run

```bash
# Development mode (backend + Vite HMR)
cd /home/bhickta/development/aicli
uv run aicli serve --data-dir ./data --dev

# Production mode (pre-built frontend)
cd frontend && npm run build && cd ..
uv run aicli serve --data-dir ./data

# Backend only
uv run aicli server --data-dir ./data --port 8765

# Frontend dev server only
cd frontend && npm run dev
```

Server starts at `http://localhost:8765` (API + UI).
Vite dev server at `http://localhost:5173` (proxies `/api` to backend).
