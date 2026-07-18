#!/usr/bin/env python3
from __future__ import annotations

import argparse
import concurrent.futures
import json
import math
import multiprocessing
import os
import re
import subprocess
import tempfile
from datetime import datetime, timezone
from pathlib import Path

from PIL import Image, ImageOps


FULL_OCR_PROMPT = (
    "Transcribe every visible text element from this page as faithfully as possible. "
    "Preserve reading order and include the page title, map title, every legible map label, "
    "place name, legend entry, caption, annotation, date, number, axis, footer, watermark "
    "text, and page number. Do not summarize, interpret, correct, or omit repeated text. "
    "Mark genuinely unreadable text as [unclear]."
)

TILE_OCR_PROMPT = (
    "Perform exhaustive OCR on this page crop. Transcribe every visible word and label "
    "exactly, especially small map labels and place names. Output plain text only, one "
    "item per line. Do not summarize, interpret, or describe the image."
)

BLOCK_RE = re.compile(
    r"^(?P<type>[A-Za-z_]+)\s+\[(?P<x1>\d+),\s*(?P<y1>\d+),\s*"
    r"(?P<x2>\d+),\s*(?P<y2>\d+)\](?P<text>.*)$"
)

_worker_provider = None


def init_aicli_worker(base_url: str, model_name: str) -> None:
    global _worker_provider
    os.environ["LM_STUDIO_BASE_URL"] = base_url
    os.environ["MODEL_NAME"] = model_name
    from aicli.providers.lm_studio import LMStudioProvider

    _worker_provider = LMStudioProvider()


def run_vision_ocr(task: tuple[str, str, int, int]) -> str:
    path, prompt, max_size, max_tokens = task
    return _worker_provider.describe_image(
        path,
        prompt,
        max_size=max_size,
        max_tokens=max_tokens,
        max_retries=2,
    )


def extract_pdf_text(pdf_path: Path, first_page: int, last_page: int) -> dict[int, str]:
    result = subprocess.run(
        [
            "pdftotext",
            "-f",
            str(first_page),
            "-l",
            str(last_page),
            "-layout",
            str(pdf_path),
            "-",
        ],
        check=True,
        capture_output=True,
    )
    pages = result.stdout.decode("utf-8", errors="replace").split("\f")
    expected = last_page - first_page + 1
    pages = pages[:expected]
    pages.extend([""] * (expected - len(pages)))
    return {
        first_page + index: text.rstrip("\n")
        for index, text in enumerate(pages)
    }


def extract_pdf_rotations(pdf_path: Path, first_page: int, last_page: int) -> dict[int, int]:
    result = subprocess.run(
        ["pdfinfo", "-f", str(first_page), "-l", str(last_page), str(pdf_path)],
        check=True,
        capture_output=True,
        text=True,
    )
    rotations: dict[int, int] = {}
    pattern = re.compile(r"^Page\s+(\d+)\s+rot:\s+(-?\d+)\s*$")
    for line in result.stdout.splitlines():
        match = pattern.match(line)
        if match:
            rotations[int(match.group(1))] = int(match.group(2)) % 360
    return rotations


def parse_layout_blocks(raw: str) -> list[dict]:
    blocks = []
    for line in raw.splitlines():
        match = BLOCK_RE.match(line.strip())
        if not match:
            continue
        blocks.append(
            {
                "type": match.group("type"),
                "bbox_normalized_1000": [
                    int(match.group("x1")),
                    int(match.group("y1")),
                    int(match.group("x2")),
                    int(match.group("y2")),
                ],
                "text": match.group("text").strip(),
            }
        )
    return blocks


def useful_tile_lines(raw: str) -> list[str]:
    ignored = {
        "[no text detected]",
        "no text detected",
        "the image contains no text.",
    }
    lines = []
    for line in raw.splitlines():
        cleaned = line.strip().strip("`")
        if cleaned and cleaned.casefold() not in ignored:
            lines.append(cleaned)
    return lines


def dedupe_lines(lines: list[str]) -> list[str]:
    seen: set[str] = set()
    result = []
    for line in lines:
        cleaned = re.sub(r"\s+", " ", line).strip()
        key = cleaned.casefold()
        if not cleaned or key in seen:
            continue
        seen.add(key)
        result.append(cleaned)
    return result


def atomic_write_json(path: Path, payload: dict) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    os.replace(temporary, path)


def page_image_path(output_dir: Path, page_number: int) -> Path:
    return output_dir / f"page-{page_number:04d}.png"


def save_model_inputs(image_path: Path, work_dir: Path, page_number: int) -> tuple[Path, list[dict]]:
    with Image.open(image_path) as opened:
        image = ImageOps.exif_transpose(opened).convert("RGB")
        width, height = image.size

        full = image.copy()
        full.thumbnail((1600, 1600), Image.Resampling.LANCZOS)
        full_path = work_dir / f"page-{page_number:04d}-full.jpg"
        full.save(full_path, format="JPEG", quality=95, subsampling=0, optimize=True)

        tile_width = min(width, math.ceil(width * 0.42))
        tile_height = min(height, math.ceil(height * 0.42))
        x_positions = [0, (width - tile_width) // 2, width - tile_width]
        y_positions = [0, (height - tile_height) // 2, height - tile_height]
        tiles = []
        for row, top in enumerate(y_positions):
            for column, left in enumerate(x_positions):
                right = left + tile_width
                bottom = top + tile_height
                tile_path = work_dir / f"page-{page_number:04d}-r{row}-c{column}.jpg"
                image.crop((left, top, right, bottom)).save(
                    tile_path,
                    format="JPEG",
                    quality=95,
                    subsampling=0,
                    optimize=True,
                )
                tiles.append(
                    {
                        "row": row,
                        "column": column,
                        "region_pixels": [left, top, right, bottom],
                        "path": tile_path,
                    }
                )
    return full_path, tiles


def build_combined_text(record: dict) -> str:
    lines = []
    pdf_text = record["ocr"]["pdf_text_layer"]
    if pdf_text.strip():
        lines.extend(pdf_text.splitlines())

    for block in record["ocr"]["full_page_vision"]["blocks"]:
        if block["text"] and block["text"] != "[Non-Text]":
            lines.append(block["text"])

    for tile in record["ocr"]["tiled_vision"]:
        lines.extend(tile["lines"])

    return "\n".join(dedupe_lines(lines))


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--pdf", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--first-page", type=int, default=1)
    parser.add_argument("--last-page", type=int, default=50)
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--manifest-name", default="ocr-manifest.json")
    parser.add_argument("--render-metadata", type=Path)
    parser.add_argument("--render-dpi", type=int, default=300)
    parser.add_argument("--full-base-url", default="http://127.0.0.1:1235/v1")
    parser.add_argument("--full-model", default="ocr-model")
    parser.add_argument(
        "--full-model-label", default="sahilchachra/Unlimited-OCR-GGUF Q8_0"
    )
    parser.add_argument("--tile-base-url", default="http://127.0.0.1:1234/v1")
    parser.add_argument("--tile-model", default="local-model")
    parser.add_argument(
        "--tile-model-label",
        default="lmstudio-community/gemma-4-12B-it-QAT-GGUF",
    )
    parser.add_argument(
        "--update-existing",
        action="store_true",
        help="Replace OCR only for the selected pages in an existing manifest.",
    )
    args = parser.parse_args()

    output_dir = args.output_dir.resolve()
    pdf_path = args.pdf.resolve()
    manifest_path = output_dir / args.manifest_name

    pdf_text = extract_pdf_text(pdf_path, args.first_page, args.last_page)
    rotations = extract_pdf_rotations(pdf_path, args.first_page, args.last_page)
    render_metadata_path = (
        args.render_metadata.resolve()
        if args.render_metadata
        else output_dir / "render-metadata.json"
    )
    if render_metadata_path.is_file():
        render_payload = json.loads(render_metadata_path.read_text(encoding="utf-8"))
        render_records = {
            record["page_number"]: record for record in render_payload.get("pages", [])
        }
    else:
        render_records = {}

    if args.update_existing:
        if not manifest_path.is_file():
            raise FileNotFoundError(manifest_path)
        payload = json.loads(manifest_path.read_text(encoding="utf-8"))
        records = {record["page_number"]: record for record in payload["pages"]}
    else:
        records: dict[int, dict] = {}

    for page_number in range(args.first_page, args.last_page + 1):
        existing = records.get(page_number)
        if args.update_existing and existing is None:
            raise KeyError(f"page {page_number} is missing from {manifest_path}")
        render_record = render_records.get(page_number, {})
        image_path = output_dir / (
            existing["image_filename"]
            if existing
            else render_record.get(
                "image_filename", page_image_path(output_dir, page_number).name
            )
        )
        if not image_path.is_file():
            raise FileNotFoundError(image_path)
        with Image.open(image_path) as image:
            dpi = image.info.get("dpi", (300, 300))
            records[page_number] = {
                "page_number": page_number,
                "image_filename": image_path.name,
                "descriptive_slug": existing.get("descriptive_slug") if existing else None,
                "image": {
                    "format": "PNG",
                    "color_mode": image.mode,
                    "width_pixels": image.width,
                    "height_pixels": image.height,
                    "dpi_x": round(float(dpi[0]), 3),
                    "dpi_y": round(float(dpi[1]), 3),
                    "source_pdf_rotation_degrees": rotations.get(page_number, 0),
                    "renderer_applied_pdf_rotation": True,
                    "post_render_rotation_degrees": render_record.get(
                        "post_render_rotation_degrees_clockwise", 0
                    ),
                    "post_render_rotation_direction": (
                        "clockwise"
                        if render_record.get("post_render_rotation_degrees_clockwise", 0)
                        in (90, 180)
                        else "counterclockwise"
                        if render_record.get("post_render_rotation_degrees_clockwise", 0) == 270
                        else "none"
                    ),
                    "orientation_policy": render_record.get(
                        "orientation_policy", "dominant-map-content-upright"
                    ),
                    "orientation_verified": render_record.get(
                        "orientation_verified", False
                    ),
                    "orientation_note": render_record.get("orientation_note"),
                },
                "ocr": {
                    "pdf_text_layer": pdf_text.get(page_number, ""),
                    "full_page_vision": {
                        "model": args.full_model_label,
                        "raw": None,
                        "blocks": [],
                        "error": None,
                    },
                    "tiled_vision": [],
                    "combined_text": "",
                },
            }

    if not args.update_existing:
        payload = {
            "schema_version": 1,
            "generated_at_utc": datetime.now(timezone.utc).isoformat(),
            "local_only": True,
            "source_pdf": str(pdf_path),
            "page_range": [args.first_page, args.last_page],
            "render": {
                "format": "PNG",
                "dpi": args.render_dpi,
                "lossless": True,
                "orientation_policy": "dominant-map-content-upright",
            },
            "ocr_sources": [
                "PDF text layer via local pdftotext",
                "full-page local Unlimited-OCR vision model",
                "overlapping 3x3 local Gemma vision OCR tiles",
            ],
        }
    payload["pages"] = [records[number] for number in sorted(records)]
    atomic_write_json(manifest_path, payload)

    context = multiprocessing.get_context("spawn")
    futures: dict[concurrent.futures.Future, tuple[str, int, dict | None]] = {}
    completed = 0

    with tempfile.TemporaryDirectory(prefix=".ocr-work-", dir=output_dir) as work:
        work_dir = Path(work)
        with concurrent.futures.ProcessPoolExecutor(
            max_workers=args.workers,
            mp_context=context,
            initializer=init_aicli_worker,
            initargs=(args.full_base_url, args.full_model),
        ) as full_pool, concurrent.futures.ProcessPoolExecutor(
            max_workers=args.workers,
            mp_context=context,
            initializer=init_aicli_worker,
            initargs=(args.tile_base_url, args.tile_model),
        ) as tile_pool:
            for page_number in range(args.first_page, args.last_page + 1):
                image_path = output_dir / records[page_number]["image_filename"]
                full_path, tiles = save_model_inputs(image_path, work_dir, page_number)
                full_future = full_pool.submit(
                    run_vision_ocr,
                    (str(full_path), FULL_OCR_PROMPT, 1600, 3072),
                )
                futures[full_future] = ("full", page_number, None)

                for tile in tiles:
                    tile_future = tile_pool.submit(
                        run_vision_ocr,
                        (str(tile["path"]), TILE_OCR_PROMPT, 1600, 256),
                    )
                    futures[tile_future] = ("tile", page_number, tile)

            total = len(futures)
            for future in concurrent.futures.as_completed(futures):
                kind, page_number, tile = futures[future]
                try:
                    raw = future.result()
                    error = None
                except Exception as exc:  # preserve failures in the manifest for retry
                    raw = ""
                    error = f"{type(exc).__name__}: {exc}"

                if kind == "full":
                    full_record = records[page_number]["ocr"]["full_page_vision"]
                    full_record["raw"] = raw
                    full_record["blocks"] = parse_layout_blocks(raw)
                    full_record["error"] = error
                else:
                    records[page_number]["ocr"]["tiled_vision"].append(
                        {
                            "row": tile["row"],
                            "column": tile["column"],
                            "region_pixels": tile["region_pixels"],
                            "model": args.tile_model_label,
                            "raw": raw,
                            "lines": useful_tile_lines(raw),
                            "error": error,
                        }
                    )

                completed += 1
                if completed % 25 == 0 or completed == total:
                    for record in records.values():
                        record["ocr"]["tiled_vision"].sort(
                            key=lambda item: (item["row"], item["column"])
                        )
                        record["ocr"]["combined_text"] = build_combined_text(record)
                    atomic_write_json(manifest_path, payload)
                    print(f"OCR progress: {completed}/{total}", flush=True)

    failures = []
    for record in records.values():
        if record["ocr"]["full_page_vision"]["error"]:
            failures.append((record["page_number"], "full"))
        for tile in record["ocr"]["tiled_vision"]:
            if tile["error"]:
                failures.append(
                    (record["page_number"], f"tile-{tile['row']}-{tile['column']}")
                )
    print(f"Manifest: {manifest_path}")
    print(f"Failures: {len(failures)}")
    if failures:
        print(json.dumps(failures))
        raise SystemExit(1)


if __name__ == "__main__":
    main()
