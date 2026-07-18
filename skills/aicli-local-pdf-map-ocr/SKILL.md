---
name: aicli-local-pdf-map-ocr
description: Render PDF map pages as high-quality lossless PNGs, visually correct map orientation, extract exhaustive auditable OCR with local AICLI/LM Studio models, and generate descriptive OCR-driven filenames. Use for PDF-to-image map extraction, NCERT or atlas page digitization, local-only OCR, rotated map pages, 3x3 tiled OCR, no-loss OCR manifests, or repeatable page-range batches that must be tested on samples before full execution.
---

# AICLI local PDF map OCR

Produce 300-DPI RGB PNGs, preserve raw OCR from three local sources, and name each image from OCR. Send only downscaled JPEGs to vision models; never replace or recompress the final PNGs with model inputs.

## Workflow

1. Confirm `pdftocairo`, `pdftotext`, `pdfinfo`, Pillow, local AICLI, and the required GGUF/mmproj files are available.
2. Render a deterministic random sample with `scripts/render_pages.py --sample-count ...`. Build a review sheet and inspect the actual map labels. Do not trust PDF rotation metadata alone.
3. Show the sample to the user and wait for approval before the full batch when the user requests staged approval.
4. Render the approved range at 300 DPI. Pass explicit clockwise/counterclockwise page lists and `--orientation-verified` only after visual verification.
5. Start Gemma vision with four 8K slots on `127.0.0.1:1234` and Unlimited-OCR on `127.0.0.1:1235`. Keep all services bound to localhost.
6. Run `scripts/ocr_pages.py`. Use one full-page Unlimited-OCR pass and nine overlapping Gemma tiles per page. Retain PDF text, raw model output, parsed blocks, crop coordinates, cleaned lines, and deduplicated combined text.
7. Restart Gemma without the vision projector, retaining four 8K slots. Run `scripts/name_pages.py`; names must come only from `ocr.combined_text`.
8. Build final contact sheets with `scripts/make_contact_sheets.py`, inspect every page, and correct/re-OCR only affected pages with `--update-existing`.
9. Mark per-page rotation metadata and run `scripts/verify_output.py`. Require zero failures before handoff.
10. Stop temporary local model servers and remove only transient work files.

## Orientation rule

Make the dominant map, its title/caption, north arrow, and place labels readable without turning the image. Some textbook leaves contain a landscape map inside a portrait book page; their book margin text cannot be upright at the same time. Preserve the full page, orient the dominant map, add a mixed-orientation note, and never crop information merely to hide sideways margins.

Use 90-degree pixel transposes, not arbitrary-angle resampling. Re-run OCR and naming after any final-image rotation because crop coordinates and reading order become stale.

## Model-server pattern

Resolve the installed `llama-server` and GGUF paths locally. Use aliases expected by the scripts:

```bash
llama-server -m GEMMA.gguf --mmproj GEMMA-mmproj.gguf \
  --host 127.0.0.1 --port 1234 --alias local-model \
  -ngl all -c 32768 -np 4 --jinja --reasoning off --no-webui

llama-server -m Unlimited-OCR.gguf --mmproj Unlimited-OCR-mmproj.gguf \
  --host 127.0.0.1 --port 1235 --alias ocr-model \
  -ngl all -c 32768 -np 4 --jinja --reasoning off --no-webui
```

Use fewer slots only if local VRAM cannot support four. Do not switch to a cloud endpoint.

## Commands

Run commands from the skill directory:

```bash
python3 scripts/render_pages.py --pdf INPUT.pdf --output-dir OUTPUT \
  --first-page 1 --last-page 50 --dpi 300 \
  --rotate-cw-pages 1,2 --mixed-orientation-pages 1,2,40 \
  --orientation-verified

python3 scripts/ocr_pages.py --pdf INPUT.pdf --output-dir OUTPUT \
  --first-page 1 --last-page 50 --workers 4

python3 scripts/name_pages.py --manifest OUTPUT/ocr-manifest.json --workers 4
python3 scripts/make_contact_sheets.py --manifest OUTPUT/ocr-manifest.json
python3 scripts/verify_output.py --manifest OUTPUT/ocr-manifest.json \
  --first-page 1 --last-page 50
```

For a corrected subset, keep the existing manifest and use `ocr_pages.py --first-page N --last-page M --update-existing`. Never rerun hundreds of pages when only a few final PNGs changed.

Read [references/manifest-schema.md](references/manifest-schema.md) when consuming or extending the JSON.

## Acceptance criteria

- Exactly the requested pages and no accidental extra batch.
- Lossless RGB PNG finals at 300 DPI with valid decodes and sensible dimensions.
- Dominant maps visually upright; every page explicitly marked orientation-verified.
- One successful full-page OCR result and nine successful tile results per page.
- Raw OCR preserved and combined OCR non-empty; no claim that OCR is infallible.
- Unique descriptive kebab-case filenames matching the manifest.
- Localhost-only model endpoints and no external data transfer.
- Contact sheets rebuilt from the current final PNGs after the last rotation/name change.
