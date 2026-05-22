# AICLI Zettel Merge

Thin Obsidian plugin for the Go-based `aicli` Zettelkasten merge engine.

The plugin is only a launcher for the inbox workflow. It sends vault settings to the local `aicli` server, where AICLI embeds the inbox note, finds semantic destination matches, and asks the merge model to return final atomic notes for existing candidate paths.

The settings include `Parallel inbox calls` and `Random inbox notes` for faster sampled inbox runs.

Default local server: `http://127.0.0.1:8765`.

Use `providerId=codex-cli` with a Codex model when you want Codex CLI / Pro for the merge call. Keep `embeddingProviderId` on a provider that supports embeddings, usually `lms` or `ollama`.

Obsidian is optional. The same workflow is available directly in the AICLI web UI under the `Zettel` tab for an AICLI-only flow.
