# aicli

`aicli` is a single-binary Go web app for controlling local, Codex, and OpenAI-compatible AI providers from one UI. It is focused on local workflows for LMS, Ollama, OpenRouter, OpenAI Codex, and custom compatible endpoints without Frappe, auth, users, or migrations for an ERP stack.

## Features

- Local web UI with chat, providers, tools, jobs, recall, and workflow screens.
- Provider support for LMS, Ollama, OpenRouter, OpenAI Codex, and custom OpenAI-compatible APIs.
- Model browser populated from the selected provider.
- Codex workflow tab backed by OpenAI's Responses API, with model filtering and reasoning/verbosity controls.
- Drag-and-drop workflow uploads stored under the app data directory.
- PDF OCR workflow with side-by-side PDF preview and final Markdown review.
- ZIP image OCR to Markdown.
- PDF analysis workflow that reuses the shared document OCR pipeline.
- Image workflows for classification, safe rename, and stale asset reference pruning.
- Audio workflows for transcription and LLM-assisted analysis.
- Video workflows for info, compression, metadata backup/restore, and notes/tags/course generation.
- News workflow for JSON/XLSX import, dedupe, merge, similarity grouping, and optional LLM cleanup.
- Zettelkasten merge workflow with local embeddings, DeepSeek-compatible judging, exact line clipping, archive, and rollback.
- Job history with status, stage, progress, elapsed time, and ETA in the UI.

## Requirements

- Go 1.24 or newer.
- One provider running or configured:
  - LMS local server, usually `http://localhost:1234/v1`.
  - Ollama, usually `http://localhost:11434`.
  - OpenRouter or another OpenAI-compatible endpoint with an API key.
  - OpenAI Codex through `OPENAI_API_KEY` or a configured provider API key.
- Optional tools, depending on workflow:
  - `pdftoppm` for PDF OCR and PDF analysis.
  - `ffmpeg` and `ffprobe` for video/audio workflows.
  - `whisper-cli` for local audio transcription.

## Run

```bash
go run ./cmd/aicli
```

Then open:

```text
http://127.0.0.1:8765
```

Useful flags:

```bash
go run ./cmd/aicli --host 127.0.0.1 --port 8080
go run ./cmd/aicli --data-dir ~/.local/share/aicli --config ~/.config/aicli/settings.json
```

Build the Vue UI bundle after frontend changes:

```bash
npm install
npm run check:web
npm run build:web
```

Build a binary:

```bash
go build -o bin/aicli ./cmd/aicli
./bin/aicli
```

## PDF OCR

1. Open `Workflows`.
2. Drag a PDF onto the drop zone.
3. Confirm the workflow switches to `OCR: PDF to Markdown`.
4. Choose a vision-capable model from LMS, Ollama, OpenRouter, or a custom provider.
5. Click `Run`.
6. Watch the stage, progress, elapsed time, and ETA.
7. Review the PDF and generated Markdown side by side.

Uploaded files are copied into:

```text
<data-dir>/uploads
```

## Zettelkasten Merge

The merge engine lives in `aicli`, not Obsidian. It scans a vault folder, builds or imports embeddings, finds similar notes, asks a local DeepSeek-compatible model to select exact source line ranges, generates a merge preview, validates it, then applies only after approval.

Default safety behavior:

- Review before apply.
- Source notes are clipped only by exact approved line ranges.
- Source notes are not deleted by default.
- Active/source files are hash-checked before writing.
- Originals are archived in `.aicli-zettel-merge/jobs/<job-id>`.
- Rollback restores the active note and source notes.

### AICLI-only workflow

1. Start `aicli`.
2. Open `http://127.0.0.1:8765`.
3. Open the `Zettel` tab.
4. Set:
   - Vault folder, for example `/home/bhickta/development/upsc`
   - Active note path, for example `zettelkasten/.../Note.md`
   - Zettelkasten folder, usually `zettelkasten`
   - Provider ID, usually `lms`
   - DeepSeek judge/merge model
   - Embedding model, usually `text-embedding-nomic-embed-text-v1.5`
5. Click `Build Index` once, or when notes/model changed.
6. Click `Suggest`.
7. Select candidate cards.
8. Click `Preview Merge`.
9. Review the final markdown.
10. Click `Apply`.

Use `Rollback` from the same tab to restore the latest applied merge, or enter a job id before rolling back.

### Optional Obsidian workflow

The plugin in `obsidian/aicli-zettel-merge` is only a thin UI over the same `aicli` APIs. Use it when you want current-note workflow inside Obsidian:

1. Copy or symlink `obsidian/aicli-zettel-merge` into the vault plugin folder.
2. Enable `AICLI Zettel Merge` in Obsidian.
3. Open a note under `zettelkasten`.
4. Run `AICLI: Suggest Zettel Merges`.
5. Select candidates, preview, apply, or rollback.

The older heavy `zettel-merge-ai` Obsidian plugin is no longer the target architecture and can be phased out after the AICLI flow is verified.

## Codex Workflow

The default settings include an `OpenAI Codex` provider:

- Provider id: `codex`
- Provider type: `openai-responses`
- Base URL: `https://api.openai.com/v1`
- API key source: `OPENAI_API_KEY`
- Model filter: `codex`

Use it from `Workflows` -> `Codex` -> `Coding task`. The model list is loaded from the provider and filtered to Codex models, so newer Codex model names can appear without a frontend change. Set `OPENAI_API_KEY` before starting `aicli`, or paste an `api_key` into the provider entry from `Settings`.

## Configuration

On first run, a default settings file is created. The UI can edit it from `Settings`.

Typical provider entries:

```json
{
  "id": "lms",
  "name": "LMS",
  "type": "openai-compatible",
  "base_url": "http://localhost:1234/v1",
  "api_key": ""
}
```

```json
{
  "id": "ollama",
  "name": "Ollama",
  "type": "ollama",
  "base_url": "http://localhost:11434",
  "api_key": ""
}
```

```json
{
  "id": "codex",
  "name": "OpenAI Codex",
  "type": "openai-responses",
  "base_url": "https://api.openai.com/v1",
  "api_key_env": "OPENAI_API_KEY",
  "model": "gpt-5.2-codex",
  "model_filter": "codex",
  "reasoning_effort": "medium",
  "text_verbosity": "medium"
}
```

## Test

```bash
npm run check:web
npm run build:web
go test ./...
```

Build verification:

```bash
go build -o /tmp/aicli-go ./cmd/aicli
```
