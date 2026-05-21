# aicli

`aicli` is a single-binary Go web app for controlling local, Codex, and OpenAI-compatible AI providers from one UI. It is focused on local workflows for LMS, Ollama, OpenRouter, OpenAI Codex, and custom compatible endpoints without Frappe, auth, users, or migrations for an ERP stack.

## Features

- Local web UI with chat, providers, tools, jobs, recall, and workflow screens.
- Provider support for LMS, Ollama, OpenRouter, OpenAI Codex API, Codex CLI / Pro, and custom OpenAI-compatible APIs.
- Model browser populated from the selected provider.
- Codex workflow tab backed by OpenAI's Responses API or the local Codex CLI, with dynamic model lists.
- Drag-and-drop workflow uploads stored under the app data directory.
- PDF OCR workflow with side-by-side PDF preview and final Markdown review.
- ZIP image OCR to Markdown.
- PDF analysis workflow that reuses the shared document OCR pipeline.
- Image workflows for classification, safe rename, and stale asset reference pruning.
- Audio workflows for transcription and LLM-assisted analysis.
- Video workflows for info, compression, metadata backup/restore, and notes/tags/course generation.
- News workflow for JSON/XLSX import, dedupe, merge, similarity grouping, and optional LLM cleanup.
- Zettelkasten merge workflow with local embeddings, provider-selectable judging/merging, exact line clipping, archive, and rollback.
- Job history with status, stage, progress, elapsed time, and ETA in the UI.

## Requirements

- Go 1.24 or newer.
- One provider running or configured:
  - LMS local server, usually `http://localhost:1234/v1`.
  - Ollama, usually `http://localhost:11434`.
  - OpenRouter or another OpenAI-compatible endpoint with an API key.
  - OpenAI Codex through `OPENAI_API_KEY`, a configured provider API key, or a logged-in `codex` CLI.
- Optional tools, depending on workflow:
  - `pdftoppm` for PDF OCR and PDF analysis.
  - `ffmpeg` and `ffprobe` for video/audio workflows.
  - `whisper-cli` for local audio transcription.
  - `codex` CLI for ChatGPT/Codex Pro plan workflows.

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

The merge engine lives in `aicli`, not Obsidian. It scans a vault folder, builds or imports embeddings, finds similar notes, asks the selected LLM provider to select exact source line ranges, generates a merge preview, validates it, then applies only after approval.

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
   - Merge provider, for example `codex-cli` for Codex CLI / Pro
   - Judge/merge model, for example `gpt-5.3-codex-spark`
   - Embedding provider, usually `lms` or `ollama`
   - Embedding model, usually `text-embedding-nomic-embed-text-v1.5`
5. Click `Build Index` once, or when notes/model changed.
6. Click `Suggest`.
7. Select candidate cards.
8. Click `Preview Merge`.
9. Review the final markdown.
10. Click `Apply`.

Use `Rollback` from the same tab to restore the latest applied merge, or enter a job id before rolling back.

### Autonomous inbox merge

For no-intervention source-note ingestion, put new atomic notes under the configured inbox folder, for example:

```text
<vault>/inbox-to-merge/**/*.md
```

Then open the `Zettel` tab and click `Run Inbox Merge`. AICLI treats inbox notes as source notes and destination notes as the configured zettelkasten folder, excluding the inbox and `.aicli-zettel-merge`. For each source note it extracts English concept units, finds destination notes through embeddings plus the merge model, asks the model for deduplicated insert actions rather than a rewritten full note, applies those actions mechanically, and moves fully processed sources into `_processed/YYYY-MM-DD/`.

The run report shows source note -> destination note mappings, merged/deduped/pending claim counts, claim ledger entries, and diffs. Facts already present are marked deduped, new facts are inserted into the best existing atomic note, and unsafe concepts stay pending. Rollback with the inbox run id restores changed destination notes and moves processed source notes back.

### Optional Obsidian workflow

The plugin in `obsidian/aicli-zettel-merge` is only a thin UI over the same `aicli` APIs. Use it when you want current-note workflow inside Obsidian:

1. Copy or symlink `obsidian/aicli-zettel-merge` into the vault plugin folder.
2. Enable `AICLI Zettel Merge` in Obsidian.
3. Open a note under `zettelkasten`.
4. Run `AICLI: Suggest Zettel Merges`.
5. Select candidates, preview, apply, or rollback.

The older heavy `zettel-merge-ai` Obsidian plugin is no longer the target architecture and can be phased out after the AICLI flow is verified.

## Codex Workflow

There are two Codex workflows:

- `Coding task (Codex CLI / Pro)` runs `codex exec` locally and uses the official Codex CLI authentication. If `codex doctor` shows `stored auth mode chatgpt`, this path uses your ChatGPT/Codex plan instead of `OPENAI_API_KEY`.
- `Coding task (API key)` calls the OpenAI API directly and uses API project billing/quota.

The CLI workflow defaults to `read-only` sandbox and `never` approval because it runs non-interactively from the web app. Switch sandbox to `workspace-write` only when you want Codex CLI to edit the selected workspace folder. Its model picker is populated by `codex debug models`, so it follows the models available to your logged-in CLI account.

Chat can also use the CLI auth path. In the `Chat` tab, select provider `Codex CLI / Pro`, refresh models, choose a model, and send the prompt. This path runs `codex exec` locally and does not require `OPENAI_API_KEY`.

The default settings also include an `OpenAI Codex` API provider:

- Provider id: `codex`
- Provider type: `openai-responses`
- Base URL: `https://api.openai.com/v1`
- API key source: `OPENAI_API_KEY`
- Model filter: `codex`

Use API-key mode from `Workflows` -> `Codex` -> `Coding task (API key)`. The model list is loaded from the provider and filtered to Codex models, so newer Codex model names can appear without a frontend change. Set `OPENAI_API_KEY` before starting `aicli`, or paste an `api_key` into the provider entry from `Settings`.

The default settings also include a CLI-backed provider:

- Provider id: `codex-cli`
- Provider type: `codex-cli`
- Tool source: `tools.codex_cli`
- Model source: `codex debug models`

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

```json
{
  "id": "codex-cli",
  "name": "Codex CLI / Pro",
  "type": "codex-cli",
  "model": "gpt-5.5"
}
```

Tool config includes:

```json
{
  "codex_cli": "codex"
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
