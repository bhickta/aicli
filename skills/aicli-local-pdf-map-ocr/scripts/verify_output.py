#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
from pathlib import Path

from PIL import Image


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Verify final PNG, OCR, naming, and orientation integrity."
    )
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--first-page", type=int, required=True)
    parser.add_argument("--last-page", type=int, required=True)
    parser.add_argument("--dpi", type=float, default=300.0)
    parser.add_argument("--tiles-per-page", type=int, default=9)
    args = parser.parse_args()

    manifest_path = args.manifest.resolve()
    output_dir = manifest_path.parent
    payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    records = payload.get("pages", [])
    errors: list[str] = []
    expected_pages = list(range(args.first_page, args.last_page + 1))
    actual_pages = [record.get("page_number") for record in records]
    if actual_pages != expected_pages:
        errors.append(f"page numbers mismatch: expected {expected_pages}, got {actual_pages}")

    filenames: set[str] = set()
    for record in records:
        page = record.get("page_number")
        filename = record.get("image_filename", "")
        if filename in filenames:
            errors.append(f"page {page}: duplicate filename {filename}")
        filenames.add(filename)
        if not re.match(rf"^page-{page:04d}-[a-z0-9]+(?:-[a-z0-9]+)+\.png$", filename):
            errors.append(f"page {page}: filename is not descriptive kebab-case: {filename}")
        path = output_dir / filename
        if not path.is_file():
            errors.append(f"page {page}: missing {path}")
            continue
        try:
            with Image.open(path) as image:
                image.verify()
            with Image.open(path) as image:
                dpi = image.info.get("dpi", (0, 0))
                if image.format != "PNG" or image.mode != "RGB":
                    errors.append(
                        f"page {page}: expected RGB PNG, got {image.mode} {image.format}"
                    )
                if min(image.size) <= 0:
                    errors.append(f"page {page}: invalid dimensions {image.size}")
                if any(abs(float(value) - args.dpi) > 1.0 for value in dpi[:2]):
                    errors.append(f"page {page}: expected {args.dpi} DPI, got {dpi}")
        except Exception as exc:
            errors.append(f"page {page}: image decode failed: {exc}")

        image_meta = record.get("image", {})
        if not image_meta.get("orientation_verified"):
            errors.append(f"page {page}: orientation is not marked verified")
        ocr = record.get("ocr", {})
        full = ocr.get("full_page_vision", {})
        if full.get("error") or not full.get("raw"):
            errors.append(f"page {page}: full-page OCR missing or failed")
        tiles = ocr.get("tiled_vision", [])
        if len(tiles) != args.tiles_per_page:
            errors.append(
                f"page {page}: expected {args.tiles_per_page} tiles, got {len(tiles)}"
            )
        for tile in tiles:
            if tile.get("error") or not tile.get("raw"):
                errors.append(
                    f"page {page}: tile {tile.get('row')},{tile.get('column')} missing or failed"
                )
        if not ocr.get("combined_text", "").strip():
            errors.append(f"page {page}: combined OCR is empty")
        if not record.get("descriptive_slug") or record.get("naming", {}).get("error"):
            errors.append(f"page {page}: naming missing or failed")

    summary = {
        "pages": len(records),
        "png_files": len(list(output_dir.glob("page-*.png"))),
        "ocr_tiles": sum(
            len(record.get("ocr", {}).get("tiled_vision", [])) for record in records
        ),
        "errors": len(errors),
    }
    print(json.dumps(summary, indent=2))
    if errors:
        for error in errors:
            print(f"ERROR: {error}")
        raise SystemExit(1)


if __name__ == "__main__":
    main()
