# AICLI Zettel Merge

Thin Obsidian plugin for the Go-based `aicli` Zettelkasten merge engine.

The plugin is only a launcher for the inbox workflow. It sends vault settings to the local `aicli` server, where AICLI embeds the inbox note, finds semantic destination matches, and asks the merge model to return final atomic notes for existing candidate paths.

The settings include `Parallel inbox calls` and `Random inbox notes` for faster sampled inbox runs.

Default local server: `http://127.0.0.1:8765`.

Use `providerId=codex-cli` with a Codex model when you want Codex CLI / Pro for the merge call. Keep `embeddingProviderId` on a provider that supports embeddings, usually `lms` or `ollama`.

Obsidian is optional. The same workflow is available directly in the AICLI web UI under the `Zettel` tab for an AICLI-only flow.

## Lecture generation

The plugin can also launch AICLI Study lecture generation from Obsidian.

Commands:

- `Generate AICLI Lecture from Active Note`
- `Generate AICLI Lecture from Active Folder`

These commands call the local AICLI endpoint `/api/workflows/study/lecture`, using the current vault path and the active note/folder as source. AICLI reads the Markdown notes, asks the configured local model to produce a crisp comprehensive lecture script, and optionally runs `ots.TTS` with the SOAR voice.

Lecture settings:

- `Lecture provider ID`
- `Lecture model`
- `Lecture style`
- `Lecture max notes`
- `Lecture max input characters`
- `Generate lecture audio`
- `TTS command`
- `TTS args`

Default TTS args:

```text
SOAR --input "{script}" --output "{audio}"
```

If `ots.TTS` is not on PATH, set `TTS command` to the absolute executable path or disable audio generation to create the script only.
