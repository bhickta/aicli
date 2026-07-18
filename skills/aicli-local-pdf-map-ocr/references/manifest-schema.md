# OCR manifest schema

The JSON manifest keeps the lossless image and every local OCR source auditable.

## Top level

- `schema_version`: schema revision.
- `generated_at_utc`: generation timestamp.
- `local_only`: confirms that no cloud model was used.
- `source_pdf`: absolute source path.
- `page_range`: inclusive processed range.
- `render`: output format, DPI, losslessness, and orientation policy.
- `ocr_sources`: ordered list of local OCR sources.
- `pages`: page records in source order.
- `naming`: local model and OCR field used for filenames.

## Page record

- `page_number`, `image_filename`, `descriptive_slug`.
- `image`: dimensions, RGB/PNG/DPI, source PDF rotation, post-render rotation,
  rotation direction, orientation policy, verification flag, and an optional mixed-orientation note.
- `ocr.pdf_text_layer`: raw local `pdftotext -layout` output.
- `ocr.full_page_vision`: model name, raw Unlimited-OCR output, parsed layout blocks, and error.
- `ocr.tiled_vision`: nine overlapping 3x3 Gemma crops. Each retains row, column,
  source-pixel region, model, raw output, cleaned lines, and error.
- `ocr.combined_text`: line-deduplicated union used for naming and search. Raw sources remain authoritative.
- `naming`: model, source field, raw response, and error.

Do not discard raw OCR when cleaning or deduplicating. `combined_text` is a convenience view,
not a replacement for the three raw sources.
