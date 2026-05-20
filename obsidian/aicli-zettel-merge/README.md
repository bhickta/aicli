# AICLI Zettel Merge

Thin Obsidian plugin for the Go-based `aicli` Zettelkasten merge engine.

The plugin does not perform embeddings, judging, merging, clipping, validation, or archive writes itself. It sends the active note path to the local `aicli` server and renders the returned candidates, merge preview, apply result, and rollback result.

Default local server: `http://127.0.0.1:8765`.

Obsidian is optional. The same workflow is available directly in the AICLI web UI under the `Zettel` tab for an AICLI-only flow.
