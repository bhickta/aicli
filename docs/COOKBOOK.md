# AICLI — Developer Cookbook

Practical recipes for common development tasks. Each recipe includes the exact files to touch, patterns to follow, and copy-paste examples.

---

## Table of Contents

1. [Add a New API Endpoint](#1-add-a-new-api-endpoint)
2. [Add a New Analysis Dimension](#2-add-a-new-analysis-dimension)
3. [Add a New Pipeline (Backend + Frontend)](#3-add-a-new-pipeline-backend--frontend)
4. [Add a New Database Column](#4-add-a-new-database-column)
5. [Add a New Frontend Tab/Workspace](#5-add-a-new-frontend-tabworkspace)
6. [Debug a Pipeline Failure](#6-debug-a-pipeline-failure)
7. [Modify an LLM Prompt](#7-modify-an-llm-prompt)
8. [Add a New Vue Component](#8-add-a-new-vue-component)
9. [Add a New CLI Command](#9-add-a-new-cli-command)
10. [Inspect and Fix Database Issues](#10-inspect-and-fix-database-issues)
11. [Add a New Video Sub-Pipeline](#11-add-a-new-video-sub-pipeline)
12. [Change the AI Provider / Model](#12-change-the-ai-provider--model)

---

## 1. Add a New API Endpoint

### Files to touch:
1. `server/schemas/{domain}_schemas.py` — Add request/response DTOs
2. `server/repositories/{domain}_repository.py` — Add data access method
3. `server/routers/{domain}.py` — Add the route

### Example: Add a "search answers" endpoint

**Step 1: Schema**
```python
# server/schemas/analyze_schemas.py
class SearchAnswersRequestDTO(BaseModel):
    query: str
    pdf_file: Optional[str] = None
```

**Step 2: Repository**
```python
# server/repositories/analyze_repository.py
def search_answers(self, query: str, pdf_file: str = None) -> List[AnswerDTO]:
    with self._db._get_conn() as conn:
        sql = "SELECT * FROM answers WHERE raw_text LIKE ?"
        params = [f"%{query}%"]
        if pdf_file:
            sql += " AND pdf_file = ?"
            params.append(pdf_file)
        rows = conn.execute(sql, params).fetchall()
        return [AnswerDTO(**dict(r)) for r in rows]
```

**Step 3: Router**
```python
# server/routers/analyze.py
@router.post("/search", response_model=List[AnswerDTO])
def search_answers(
    req: SearchAnswersRequestDTO,
    repo: AnalyzeRepository = Depends(get_analyze_repository),
):
    return repo.search_answers(req.query, req.pdf_file)
```

### Pattern rules:
- Routers import from `schemas/` and `repositories/`
- Business logic stays out of routers — delegate to service or repository
- Use `Depends()` for dependency injection

---

## 2. Add a New Analysis Dimension

**This requires ZERO code changes.** Just edit the YAML config.

### File to touch:
- `domains/analyze/prompts.yaml`

### Steps:

Add under the `dimensions:` section:

```yaml
dimensions:
  # ... existing dimensions ...

  vocabulary:
    enabled: true
    prompt: |
      Analyze the vocabulary sophistication of this UPSC answer.

      Answer text:
      ---
      {answer_text}
      ---

      Return strict JSON:
      {
        "sophistication_level": "basic|intermediate|advanced",
        "technical_terms": [],
        "academic_phrases": [],
        "total_unique_words": 0,
        "avg_word_length": 0.0
      }

      Return ONLY valid JSON, no other text.
```

Then reset and re-run steps 5+ from the UI or API:

```bash
curl -X POST http://localhost:8765/api/analyze/reset -H 'Content-Type: application/json' -d '{"step": 5}'
curl -X POST http://localhost:8765/api/analyze/run -H 'Content-Type: application/json' -d '{"target_steps": [5, 6]}'
```

### How it works internally:
- `AnalyzeConfig` loads `prompts.yaml` and exposes `enabled_dimensions`
- `DimensionAnalyzerService.analyze_answer()` reads the prompt template, substitutes `{answer_text}`, and calls the LLM
- Results get stored in `answer_dimensions` table with `dimension_name = "vocabulary"`

---

## 3. Add a New Pipeline (Backend + Frontend)

### Backend files to create/modify:

1. **`server/schemas/new_schemas.py`** — Request DTO
2. **`server/pipelines/new_pipeline.py`** — Pipeline logic (or reuse existing service)
3. **`server/routers/new_router.py`** — Router with `/run` + `/stream`
4. **`server/app.py`** — Register the router

### Example: Add an "Audio" pipeline

**Step 1: Schema**
```python
# server/schemas/audio_schemas.py
from pydantic import BaseModel

class AudioTranscribeRequestDTO(BaseModel):
    target_path: str
    whisper_model: str = "base"
    output_format: str = "srt"
```

**Step 2: Pipeline module**
```python
# server/pipelines/audio.py
from pathlib import Path
from aicli.cli.tui import console

def transcribe_audio(target_path: Path, whisper_model: str, output_format: str):
    console.print(f"Transcribing {target_path}...")
    # ... your logic here ...
    console.print("Done!")
```

**Step 3: Router**
```python
# server/routers/audio.py
from fastapi import APIRouter, HTTPException
from fastapi.responses import EventSourceResponse
from aicli.server.orchestrator.base import BaseOrchestrator
from aicli.server.orchestrator.console_patcher import ConsolePatcher
from aicli.server.dependencies import ServerState
from aicli.server.schemas.audio_schemas import AudioTranscribeRequestDTO

router = APIRouter()
audio_orch = BaseOrchestrator()

def _audio_worker(orch: BaseOrchestrator, req: AudioTranscribeRequestDTO):
    import aicli.server.pipelines.audio as audio_mod
    target = ServerState.data_dir / req.target_path if not Path(req.target_path).is_absolute() else Path(req.target_path)
    with ConsolePatcher(audio_mod, orch.queue):
        audio_mod.transcribe_audio(target, req.whisper_model, req.output_format)

@router.post("/transcribe/run")
def run_audio_transcribe(req: AudioTranscribeRequestDTO):
    try:
        audio_orch.dispatch(_audio_worker, req=req)
        return {"ok": True, "message": "Audio Transcribe Pipeline started"}
    except RuntimeError as e:
        raise HTTPException(status_code=409, detail=str(e))

@router.get("/stream")
async def stream_audio():
    return EventSourceResponse(audio_orch.stream_events())
```

**Step 4: Register in app.py**
```python
# server/app.py — add these lines:
from aicli.server.routers.audio import router as audio_router
app.include_router(audio_router, prefix="/api/audio", tags=["Audio"])
```

### Frontend files to create:

1. **`frontend/src/components/AudioStudio/AudioSidebar.vue`**
2. **`frontend/src/components/AudioStudio/TranscribePanel.vue`**
3. **`frontend/src/components/AudioStudio.vue`** — Wrapper
4. **`frontend/src/composables/useAudioPipeline.ts`** — Uses `useStreamPipeline`

**Composable (follows existing pattern):**
```typescript
// frontend/src/composables/useAudioPipeline.ts
import { useStreamPipeline } from './useStreamPipeline'
import { API_BASE } from '../constants/api.constants'

export function useAudioPipeline() {
  return useStreamPipeline({
    buildPostUrl: (endpoint) => `${API_BASE}/api/audio/${endpoint}`,
    streamUrl: `${API_BASE}/api/audio/stream`,
    pipelineName: 'Audio',
    validate: (config) => !!config.target_path,
    validationMessage: 'Please provide a target path.',
  })
}
```

**Register in App.vue:**
```vue
<!-- Add to App.vue nav-links -->
<button :class="['nav-btn', { active: workspace === 'audio' }]"
        @click="workspace = 'audio'" title="Audio Studio">🎵</button>

<!-- Add to workspace-container -->
<AudioStudio v-show="workspace === 'audio'" />
```

---

## 4. Add a New Database Column

### Files to touch:
1. `domains/analyze/db/base.py` — Add column to `CREATE TABLE`
2. Relevant mixin in `domains/analyze/db/` — Add query methods
3. `server/schemas/analyze_schemas.py` — Add to DTO
4. `server/repositories/analyze_repository.py` — Update if needed

### Example: Add `marks_scored` to answers

**Step 1: Schema migration (SQLite)**
```sql
-- Run manually or add to base.py's _create_tables
ALTER TABLE answers ADD COLUMN marks_scored REAL;
```

**Step 2: Update the CREATE TABLE** (for fresh installs)
```python
# domains/analyze/db/base.py — inside _create_tables
# Add to the answers table definition:
# marks_scored REAL,
```

**Step 3: Update the DTO**
```python
# server/schemas/analyze_schemas.py
class AnswerDTO(BaseModel):
    # ... existing fields ...
    marks_scored: Optional[float] = None
```

**Step 4: Update the mixin** (if you need to query by it)
```python
# domains/analyze/db/answers.py
def update_marks(self, answer_id: int, marks: float):
    conn = self._get_conn()
    conn.execute("UPDATE answers SET marks_scored = ? WHERE id = ?", (marks, answer_id))
    conn.commit()
```

> **IMPORTANT:** SQLite doesn't support `DROP COLUMN`. For column removal, you'd need to recreate the table. For additions, `ALTER TABLE ADD COLUMN` works fine.

---

## 5. Add a New Frontend Tab/Workspace

### Files to touch:
1. `frontend/src/components/NewStudio.vue` — Create the component
2. `frontend/src/App.vue` — Add nav button + render

### Pattern:
```vue
<!-- frontend/src/components/NewStudio.vue -->
<template>
  <div class="workspace-layout">
    <aside class="sidebar">
      <!-- sidebar content -->
    </aside>
    <main class="main-content">
      <!-- main content -->
    </main>
  </div>
</template>

<script setup>
// composables, state, etc.
</script>

<style>
/* Use tokens from styles/tokens.css */
.workspace-layout { display: flex; height: 100%; width: 100%; }
.sidebar {
  width: 280px;
  background: var(--bg-card);
  border-right: 1px solid var(--border);
  overflow-y: auto;
}
.main-content { flex: 1; overflow-y: auto; padding: 24px; }
</style>
```

---

## 6. Debug a Pipeline Failure

### Step-by-step debugging:

**1. Check the SSE logs in the browser console**
The frontend logs all SSE events. Look for `[SYSTEM ERROR]` messages.

**2. Check the server terminal**
The FastAPI server prints exceptions from background threads.

**3. Check the database for errors**
```sql
-- Transcription errors
SELECT id, pdf_file, page_number, transcription 
FROM pages WHERE transcription LIKE '[TRANSCRIPTION_ERROR%';

-- Processing log
SELECT * FROM processing_log WHERE status = 'error' ORDER BY timestamp DESC LIMIT 10;
```

**4. Retry failed items**
```bash
# Via API
curl -X POST http://localhost:8765/api/analyze/retry-errors

# Via SQL
sqlite3 ./data/analyze.db "UPDATE pages SET transcription = NULL, processed = 0 WHERE transcription LIKE '[TRANSCRIPTION_ERROR%'"
```

**5. Reset and re-run a specific step**
```bash
# Reset from step 3 (classification) — clears steps 3, 4, 5, 6
curl -X POST http://localhost:8765/api/analyze/reset \
  -H 'Content-Type: application/json' -d '{"step": 3}'

# Run only step 3
curl -X POST http://localhost:8765/api/analyze/run \
  -H 'Content-Type: application/json' -d '{"target_steps": [3]}'
```

**6. Re-run a single page**
```bash
curl -X POST http://localhost:8765/api/analyze/run \
  -H 'Content-Type: application/json' -d '{"page_id": 42, "target_steps": [2, 3, 4, 5]}'
```

### Common failure causes:
| Symptom | Cause | Fix |
|---------|-------|-----|
| `TRANSCRIPTION_ERROR` | LM Studio timeout or OOM | Reduce image size, restart LM Studio |
| Empty answers list | Segmentation LLM returned invalid JSON | Reset step 4, re-run |
| Pipeline stuck at "running" | Thread crashed silently | Restart server, check logs |
| `409 Conflict` on `/run` | Previous pipeline still running | Wait or restart server |
| SSE stream disconnects | Server restart or network issue | Reconnect in UI |

---

## 7. Modify an LLM Prompt

### File: `domains/analyze/prompts.yaml`

All prompts are in this single YAML file. Edit them directly — no code changes needed.

### Prompt template variables:
- `{pages_text}` — Used in segmentation prompt (injected by `AnswerSegmenterService`)
- `{answer_text}` — Used in dimension prompts (injected by `DimensionAnalyzerService`)
- `{count}`, `{candidates}`, `{dimension_name}`, `{dimension_data}` — Used in aggregation prompt

### LM Studio settings (also in prompts.yaml):
```yaml
lm_studio:
  max_tokens: 4096        # Maximum response length
  temperature: 0.0        # Deterministic output
  max_retries: 3          # Retry on failure
  retry_backoff_base: 2   # Exponential backoff base
  image_max_size: 2048    # Max image dimension for vision
```

---

## 8. Add a New Vue Component

### Pattern: Follow the existing decomposition

```
components/
└── DomainStudio/         ← Directory per major feature
    ├── FeaturePanel.vue  ← One component per panel/feature
    └── FeatureSidebar.vue
```

### Component template:
```vue
<template>
  <div class="feature-panel">
    <h3>Feature Title</h3>
    <!-- Use .form-group, .btn, .btn-primary from tokens.css -->
    <div class="form-group">
      <label>Some Input</label>
      <input v-model="inputValue" />
    </div>
    <button class="btn btn-primary" @click="doThing" :disabled="running">
      {{ running ? 'Running...' : 'Start' }}
    </button>
  </div>
</template>

<script setup>
import { ref } from 'vue'

const inputValue = ref('')
const running = ref(false)

async function doThing() {
  running.value = true
  try {
    // use composable or API client
  } finally {
    running.value = false
  }
}
</script>
```

### CSS classes available globally:
- `.btn`, `.btn-primary`, `.btn-danger`, `.btn-ghost`, `.btn-sm`
- `.form-group` (with `label`, `input`, `select`)
- `.classification-badge`, `.badge-answer`, `.badge-error`, etc.
- `.empty-state` with `.icon`
- `.loading`, `.spinner`
- `.tabs`, `.tab`, `.tab.active`

---

## 9. Add a New CLI Command

### Files to touch:
1. `cli/commands/new_command.py` — Create the command module
2. `main.py` — Register the command group

### Example: Add a "db" command group

```python
# cli/commands/db.py
import typer
from pathlib import Path

app = typer.Typer(help="Database inspection commands.")

@app.command()
def status(
    data_dir: Path = typer.Option("./data", "--data-dir", "-d"),
):
    """Show database status summary."""
    from aicli.domains.analyze.database import AnalyzeDB
    db = AnalyzeDB(data_dir / "analyze.db")
    status = db.get_processing_status()
    typer.echo(f"PDFs: {status['total_pdfs']}")
    typer.echo(f"Pages: {status['total_pages']}")
    typer.echo(f"Answers: {status['total_answers']}")
```

```python
# main.py — register it
from aicli.cli.commands import db
app.add_typer(db.app, name="db", help="Database inspection commands")
```

---

## 10. Inspect and Fix Database Issues

### Quick inspection script (Python)

```python
from pathlib import Path
from aicli.domains.analyze.database import AnalyzeDB

db = AnalyzeDB(Path("./data/analyze.db"))

# Check status
print(db.get_processing_status())

# List all PDFs
print(db.get_all_pdfs())

# Get pages for a PDF
pages = db.get_pages_for_pdf("example.pdf")
print(f"Pages: {len(pages)}")

# Get untranscribed pages
untranscribed = db.get_untranscribed_pages()
print(f"Untranscribed: {len(untranscribed)}")

# Get unclassified pages
unclassified = db.get_unclassified_pages()
print(f"Unclassified: {len(unclassified)}")

# Get unsegmented PDFs
unsegmented = db.get_unsegmented_pdfs()
print(f"Unsegmented PDFs: {unsegmented}")

db.close()
```

### Common fixes via SQL

```sql
-- Fix a misclassified page
UPDATE pages SET classification = 'answer' WHERE id = 42;

-- Delete answers for re-segmentation of one PDF
DELETE FROM answer_dimensions WHERE answer_id IN (SELECT id FROM answers WHERE pdf_file = 'bad.pdf');
DELETE FROM answers WHERE pdf_file = 'bad.pdf';

-- Wipe the entire database and start fresh
DELETE FROM processing_log;
DELETE FROM dimension_aggregations;
DELETE FROM answer_dimensions;
DELETE FROM answers;
DELETE FROM pages;
```

---

## 11. Add a New Video Sub-Pipeline

### Follow the existing pattern exactly:

1. **Create pipeline module:** `server/pipelines/video_newfeature.py`
2. **Add schema:** `server/schemas/video_schemas.py`
3. **Add worker + endpoint:** `server/routers/video.py`

```python
# server/schemas/video_schemas.py — add DTO
class VideoNewFeatureRequestDTO(BaseModel):
    target_path: str
    some_option: str = "default"

# server/routers/video.py — add worker + endpoint
def _video_new_worker(orch: BaseOrchestrator, req: VideoNewFeatureRequestDTO):
    import aicli.server.pipelines.video_newfeature as new_mod
    target = _resolve_path(req.target_path)
    with ConsolePatcher(new_mod, orch.queue):
        new_mod.do_thing(target, req.some_option)

@router.post("/newfeature/run")
def run_video_new(req: VideoNewFeatureRequestDTO):
    return _dispatch(video_orch, _video_new_worker, "Video NewFeature Pipeline", req=req)
```

**Note:** All video sub-pipelines share the **same orchestrator** (`video_orch`) and SSE stream (`/api/video/course/stream`).

---

## 12. Change the AI Provider / Model

### Runtime model switching

```bash
# Via UI: Settings tab → change model_name
# Via API:
curl -X POST http://localhost:8765/api/settings \
  -H 'Content-Type: application/json' \
  -d '{"lm_studio_base_url": "http://localhost:1234/v1", "model_name": "qwen2-vl-7b"}'
```

### Per-pipeline model override

The analyze pipeline accepts `llm_model` in its request:
```bash
curl -X POST http://localhost:8765/api/analyze/run \
  -H 'Content-Type: application/json' \
  -d '{"llm_model": "gemma-4-26b-a4b", "workers": 4}'
```
**Note:** When the pipeline runs (either via CLI or `/api/analyze/run`), it natively intercepts the request and automatically forces LM Studio to load the requested model into VRAM via `/api/v1/models/load` before any background worker begins. Heavy MoE models are booted using parallelized configurations natively.

### Adding a new AI provider

1. Create `providers/new_provider.py` implementing `ImageVisionProvider`:
   ```python
   from aicli.core.interfaces import ImageVisionProvider

   class NewProvider(ImageVisionProvider):
       def describe_image(self, image_path, prompt, system_prompt=None, allow_reasoning=True) -> str:
           ...
       def complete_text(self, prompt, system_prompt=None, allow_reasoning=True) -> str:
           ...
       def complete_text_json(self, prompt, system_prompt=None, allow_reasoning=True) -> dict:
           ...
   ```
2. Update `server/dependencies.py` to inject the new provider instead of `LMStudioProvider`
