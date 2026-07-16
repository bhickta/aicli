# aicli

`aicli` is a single-binary Go web app for controlling local, Codex, and OpenAI-compatible AI providers from one UI. It is focused on local workflows for LMS, Ollama, OpenRouter, OpenAI Codex, and custom compatible endpoints without ERP user or session dependencies. Its service execution API uses bearer-token authentication when Frappe connects to it.

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
- Zettelkasten inbox merge workflow with local embeddings, semantic destination search, one AI final-note merge call, archive, and rollback.
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

Set a service token when Frappe will use the authenticated execution API:

```bash
AICLI_SERVICE_TOKEN="$(openssl rand -hex 32)" go run ./cmd/aicli
```

Use the same token in Frappe's `AI Control Settings` or its `AICLI_SERVICE_TOKEN` environment variable. The browser must never call this API or receive the service token.

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

## Zettelkasten Inbox Merge

The inbox merge engine lives in `aicli`, not Obsidian. The runtime flow is intentionally small:

1. Embed the inbox source note.
2. Find semantically similar destination notes from the zettelkasten index.
3. Ask the merge model to return complete final atomic destination notes for those candidates.

Use `Run size` to limit a test run, `Random notes` to sample from the inbox instead of always taking the first sorted notes, and `Parallel calls` to run multiple AI merge calls at once. Destination writes are still serialized so parallel runs do not overwrite each other.

1. Start `aicli`.
2. Open `http://127.0.0.1:8765`.
3. Open the `Zettel` tab.
4. Set:
   - Vault folder, for example `/home/bhickta/development/upsc`
   - Source inbox folder, for example `inbox-to-merge`
   - Zettelkasten folder, usually `zettelkasten`
   - AI merge provider and model
   - Embedding provider, usually `lms` or `ollama`
   - Embedding model, usually `text-embedding-nomic-embed-text-v1.5`
5. Click `Build Index` once, or when notes/model changed.
6. Click `Preview Embedding Matches` to inspect the destination files selected by semantic search.
7. Click `Run Inbox Merge`.

For no-intervention source-note ingestion, put new atomic notes under the configured inbox folder, for example:

```text
<vault>/inbox-to-merge/**/*.md
```

Then open the `Zettel` tab and click `Run Inbox Merge`. AICLI treats inbox notes as source notes and destination notes as the configured zettelkasten folder, excluding the inbox and `.aicli-zettel-merge`. For each source note it embeds the source, finds semantically similar destination notes, then asks the merge model to choose candidate paths and return complete atomic destination notes in `BEGIN_NOTE` / `END_NOTE` blocks. Fully processed sources move into `_processed/YYYY-MM-DD/`.

The run report shows source note -> destination note mappings, merged/deduped/pending counts, diffs, provider API calls, and the rollback id. If the model returns `PENDING` or writes only non-candidate paths, the source note stays in place. Rollback with the inbox run id restores changed destination notes and moves processed source notes back.

### Clean merge training data

The `Zettel` -> `Training` tab exports clean chat-SFT JSONL from saved inbox merge audit data. It is local-only and makes zero provider/API calls.

The exporter includes only successful final merge examples:

```text
workflow = zettel-inbox-merge
step = merge-final-notes
parsed_format = final-notes
error = empty
```

Strict mode is enabled by default. It also removes examples that would teach bad behavior:

- old prompt variants
- missing semantic destination candidates
- assistant markdown code fences
- duplicate frontmatter
- malformed `BEGIN_NOTE` / `END_NOTE` boundaries
- status/JSON responses

It skips failed, pending, unparsed, metadata, judge, validation, and duplicate examples, then writes:

```text
<aicli-data-dir>/zettel/<vault-key>/training-exports/<run-id>/train.jsonl
<aicli-data-dir>/zettel/<vault-key>/training-exports/<run-id>/eval.jsonl
<aicli-data-dir>/zettel/<vault-key>/training-exports/<run-id>/train.sharegpt.jsonl
<aicli-data-dir>/zettel/<vault-key>/training-exports/<run-id>/eval.sharegpt.jsonl
<aicli-data-dir>/zettel/<vault-key>/training-exports/<run-id>/manifest.json
```

Use `train.jsonl` and `eval.jsonl` for chat-SFT trainers, or the `*.sharegpt.jsonl` files for ShareGPT-style trainers such as common Axolotl/Unsloth recipes. Start with a 7B/8B instruct model on an RTX 3090. Metadata examples are intentionally excluded from this export so the model learns the merge task only. The manifest includes quality counters; for a strict export, red-flag counters should be zero before training.

You can also export without starting the web server:

```bash
go run ./cmd/aicli zettel-training-export \
  -vault /path/to/vault \
  -data-folder /path/to/aicli/zettel/<vault-key> \
  -strict=true
```

### Optional Obsidian workflow

The plugin in `obsidian/aicli-zettel-merge` is a thin launcher over the same inbox flow:

1. Copy or symlink `obsidian/aicli-zettel-merge` into the vault plugin folder.
2. Enable `AICLI Zettel Merge` in Obsidian.
3. Configure the vault, inbox folder, destination folder, merge model, and embedding model.
4. Run `AICLI: Build AICLI Zettel Index` when notes/model changed.
5. Run `AICLI: Run AICLI Inbox Merge`.

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

### Frappe execution API

`AICLI_SERVICE_TOKEN` enables the Bearer-authenticated control and execution endpoints:

- `POST /api/execution/run` executes text, structured, vision, OCR, embedding, or rerank profiles.
- `GET /api/execution/control` returns sanitized providers and execution profiles.
- `PUT /api/execution/providers` and `PUT /api/execution/profiles` update execution configuration.
- `GET /api/execution/models` and `POST /api/execution/health` inspect a provider.

Profiles own model order, fallback, concurrency, timeout, cooldown, and optional per-million-token rates. Frappe owns feature routes, prompts, budgets, audit logs, jobs, permissions, and persistence.

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
