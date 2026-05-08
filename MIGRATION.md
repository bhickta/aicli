# AICLI Go Migration Tracker

The Go app is intended to replace the Frappe implementation with a local single-binary control center. This tracker keeps parity work explicit.

## Implemented

- Single Go binary entrypoint at `cmd/aicli`.
- Embedded local web UI.
- JSON settings with LM Studio, Ollama, OpenRouter, and custom OpenAI-compatible provider shape.
- SQLite job store.
- Health/settings/provider/model/chat/job APIs.
- LM Studio/OpenAI-compatible chat and model-list adapter.
- Ollama chat and model-list adapter.
- Tool readiness API for FFmpeg, FFprobe, Poppler `pdftoppm`, and `whisper-cli`.
- UPSC Recall workflow with job persistence.
- Image workflow for rename, junk detection, and Markdown digitization through vision models.
- OCR workflow for ZIP image batches to ordered Markdown.
- Analyze workflow for PDF to page images, vision OCR, and Markdown report generation.
- Audio transcription workflow through `whisper-cli`.
- Video info and compression workflows through `ffprobe` and `ffmpeg`.
- News JSON dedupe workflow with optional LLM merge summary.
- Audio LLM analysis and playlist grouping.
- Video notes, tags, and course-plan generation.
- News XLSX import/export.
- Image safe on-disk rename and stale reference pruning.
- Provider-level chat streaming with SSE endpoint.
- Deterministic similarity clustering for news items.
- Video metadata backup and restore through FFmpeg metadata sidecars.
- Unit tests for config, providers, storage, HTTP handlers, tool helpers, and migrated workflows.

## Remaining Polish

- Optional frontend conversion from the current static shell to a full Vue workspace or Wails wrapper.

## Quality Gates

- Every workflow starts with a failing unit test around observable behavior.
- External tools are wrapped behind interfaces and tested with fakes.
- Real LM Studio/Ollama/OpenRouter/tool tests must be build-tagged as integration tests.
- `go test ./...` must pass before each committed slice.
