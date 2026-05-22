# AICLI Zettel Merge

Thin Obsidian plugin for the Go-based `aicli` Zettelkasten merge engine.

The plugin is only a launcher for the inbox workflow. It sends vault settings to the local `aicli` server, where AICLI embeds the inbox note, finds semantic destination matches, judges the safest targets, merges final atomic notes, and validates them before writing.

Default local server: `http://127.0.0.1:8765`.

Use `providerId=codex-cli` with a Codex model when you want Codex CLI / Pro for the judge, merge, and validation calls. Keep `embeddingProviderId` on a provider that supports embeddings, usually `lms` or `ollama`.

Obsidian is optional. The same workflow is available directly in the AICLI web UI under the `Zettel` tab for an AICLI-only flow.
