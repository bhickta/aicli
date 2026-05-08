# aicli

`aicli` is a single-binary Go web app for controlling local and OpenAI-compatible AI providers from one UI. It is focused on local workflows for LMS, Ollama, OpenRouter, and custom compatible endpoints without Frappe, auth, users, or migrations for an ERP stack.

## Features

- Local web UI with chat, providers, tools, jobs, recall, and workflow screens.
- Provider support for LMS, Ollama, OpenRouter, and custom OpenAI-compatible APIs.
- Model browser populated from the selected provider.
- Drag-and-drop workflow uploads stored under the app data directory.
- PDF OCR workflow with side-by-side PDF preview and final Markdown review.
- ZIP image OCR to Markdown.
- PDF analysis workflow that reuses the shared document OCR pipeline.
- Image workflows for classification, safe rename, and stale asset reference pruning.
- Audio workflows for transcription and LLM-assisted analysis.
- Video workflows for info, compression, metadata backup/restore, and notes/tags/course generation.
- News workflow for JSON/XLSX import, dedupe, merge, similarity grouping, and optional LLM cleanup.
- Job history with status, stage, progress, elapsed time, and ETA in the UI.

## Requirements

- Go 1.24 or newer.
- One provider running or configured:
  - LMS local server, usually `http://localhost:1234/v1`.
  - Ollama, usually `http://localhost:11434`.
  - OpenRouter or another OpenAI-compatible endpoint with an API key.
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
http://127.0.0.1:8080
```

Useful flags:

```bash
go run ./cmd/aicli --host 127.0.0.1 --port 8080
go run ./cmd/aicli --data-dir ~/.local/share/aicli --config ~/.config/aicli/settings.json
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

## Configuration

On first run, a default settings file is created. The UI can edit it from `Settings`.

Typical provider entries:

```json
{
  "id": "lms",
  "name": "LMS",
  "type": "openai",
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

## Test

```bash
go test ./...
```

Build verification:

```bash
go build -o /tmp/aicli-go ./cmd/aicli
```
