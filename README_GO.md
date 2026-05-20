# AICLI Go Migration

This is the standalone Go replacement for the Frappe-based AICLI prototype.

## Run

```bash
go run ./cmd/aicli --open
```

The binary starts a local web UI at `http://127.0.0.1:8765`, stores settings in the data directory, and stores job state in SQLite.

External tools are detected at runtime. On this machine, FFmpeg, FFprobe, and Poppler `pdftoppm` are available; `whisper-cli` must be installed or configured before audio transcription can run, and `codex` must be installed/logged in before Codex Pro workflows can run.

## Current Slice

- Single Go entrypoint.
- Embedded local web UI.
- JSON settings.
- SQLite job storage.
- LMS/OpenAI-compatible provider adapter.
- Ollama provider adapter.
- OpenRouter/custom endpoint support through the OpenAI-compatible adapter.
- OpenAI Codex provider support through the Responses API, including Codex model filtering and reasoning controls.
- Codex CLI provider support for ChatGPT/Codex-authenticated chat and workflows that do not use `OPENAI_API_KEY`, with model lists loaded from `codex debug models`.
- Health, settings, provider, chat, and jobs APIs.
- Tool readiness checks for `ffmpeg`, `ffprobe`, `pdftoppm`, `whisper-cli`, and `codex`.
- UPSC Recall workflow.
- Codex coding workflows for API-key and Codex CLI modes.
- Image rename/junk/digitize workflow.
- OCR ZIP-to-Markdown workflow.
- Analyze PDF-to-report workflow.
- Audio transcription workflow.
- Audio LLM analysis and playlist grouping workflow.
- Video info/compress workflow.
- Video notes/tags/course workflow.
- Video metadata backup/restore workflow.
- News JSON/XLSX dedupe workflow.
- News similarity clustering.
- Image safe rename and stale reference pruning.
- Vue 3 web UI served from the embedded Go binary.
- AICLI-owned Zettelkasten merge workflow with separate LLM and embedding providers, exact line clipping, validation, archive, and rollback.

## Remaining Polish

Core local backend workflows are represented as Go use cases. Remaining polish is optional workflow depth, packaging, and richer job-specific review screens where useful.
