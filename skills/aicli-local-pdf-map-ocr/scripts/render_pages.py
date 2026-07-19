#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import random
import re
import subprocess
import tempfile
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from pathlib import Path

from PIL import Image, ImageOps


def parse_pages(value: str) -> set[int]:
    pages: set[int] = set()
    if not value.strip():
        return pages
    for item in value.split(","):
        item = item.strip()
        if not item:
            continue
        if "-" in item:
            start_text, end_text = item.split("-", 1)
            start, end = int(start_text), int(end_text)
            if start > end:
                raise ValueError(f"invalid page range: {item}")
            pages.update(range(start, end + 1))
        else:
            pages.add(int(item))
    if any(page < 1 for page in pages):
        raise ValueError("page numbers must be positive")
    return pages


def pdf_page_count(pdf_path: Path) -> int:
    result = subprocess.run(
        ["pdfinfo", str(pdf_path)], check=True, capture_output=True, text=True
    )
    match = re.search(r"^Pages:\s+(\d+)\s*$", result.stdout, flags=re.MULTILINE)
    if not match:
        raise RuntimeError("pdfinfo did not report a page count")
    return int(match.group(1))


def pdf_rotations(pdf_path: Path, first_page: int, last_page: int) -> dict[int, int]:
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


def rotate_clockwise(image: Image.Image, degrees: int) -> Image.Image:
    normalized = degrees % 360
    operations = {
        0: None,
        90: Image.Transpose.ROTATE_270,
        180: Image.Transpose.ROTATE_180,
        270: Image.Transpose.ROTATE_90,
    }
    if normalized not in operations:
        raise ValueError("rotation must be a multiple of 90 degrees")
    operation = operations[normalized]
    return image.copy() if operation is None else image.transpose(operation)


def atomic_json(path: Path, payload: dict) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    os.replace(temporary, path)


def render_page(
    *,
    pdf_path: Path,
    output_dir: Path,
    work_dir: Path,
    page_number: int,
    target_name: str,
    dpi: int,
    post_rotation: int,
    source_rotation: int,
    orientation_verified: bool,
    mixed_orientation: bool,
) -> dict:
    target = output_dir / target_name
    prefix = work_dir / f"page-{page_number:04d}"
    subprocess.run(
        [
            "pdftocairo",
            "-png",
            "-singlefile",
            "-r",
            str(dpi),
            "-f",
            str(page_number),
            "-l",
            str(page_number),
            str(pdf_path),
            str(prefix),
        ],
        check=True,
    )
    rendered = prefix.with_suffix(".png")
    with Image.open(rendered) as opened:
        image = ImageOps.exif_transpose(opened).convert("RGB")
        image = rotate_clockwise(image, post_rotation)
        temporary_target = target.with_suffix(target.suffix + ".tmp")
        image.save(
            temporary_target,
            format="PNG",
            dpi=(dpi, dpi),
            compress_level=6,
        )
        os.replace(temporary_target, target)
        width, height = image.size

    if mixed_orientation:
        note = (
            "Dominant map content is upright; source book margin/header text "
            "remains sideways by design."
        )
    else:
        note = None
    return {
        "page_number": page_number,
        "image_filename": target.name,
        "width_pixels": width,
        "height_pixels": height,
        "dpi_x": dpi,
        "dpi_y": dpi,
        "source_pdf_rotation_degrees": source_rotation,
        "renderer_applied_pdf_rotation": True,
        "post_render_rotation_degrees_clockwise": post_rotation,
        "orientation_policy": "dominant-map-content-upright",
        "orientation_verified": orientation_verified,
        "orientation_note": note,
    }


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Render selected PDF pages to lossless RGB PNGs with explicit rotation overrides."
    )
    parser.add_argument("--pdf", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    parser.add_argument("--first-page", type=int, default=1)
    parser.add_argument("--last-page", type=int)
    parser.add_argument("--pages", default="", help="Comma/range list, e.g. 1,7,12-15")
    parser.add_argument(
        "--sample-count",
        type=int,
        default=0,
        help="Randomly select this many pages from the requested range.",
    )
    parser.add_argument("--seed", type=int, default=20260718)
    parser.add_argument("--dpi", type=int, default=300)
    parser.add_argument(
        "--workers",
        type=int,
        default=1,
        help="Number of pages to render concurrently (default: 1).",
    )
    parser.add_argument("--rotate-cw-pages", default="")
    parser.add_argument("--rotate-ccw-pages", default="")
    parser.add_argument("--rotate-180-pages", default="")
    parser.add_argument(
        "--mixed-orientation-pages",
        default="",
        help="Pages whose dominant map and book margins cannot both be upright.",
    )
    parser.add_argument("--orientation-verified", action="store_true")
    parser.add_argument("--overwrite", action="store_true")
    args = parser.parse_args()

    if args.workers < 1:
        parser.error("--workers must be at least 1")

    pdf_path = args.pdf.resolve()
    output_dir = args.output_dir.resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    total_pages = pdf_page_count(pdf_path)

    explicit_pages = parse_pages(args.pages)
    last_page = args.last_page or total_pages
    requested = explicit_pages or set(range(args.first_page, last_page + 1))
    if not requested or min(requested) < 1 or max(requested) > total_pages:
        raise ValueError(f"requested pages must fall within 1-{total_pages}")
    if args.sample_count:
        if args.sample_count > len(requested):
            raise ValueError("sample count exceeds requested page count")
        requested = set(random.Random(args.seed).sample(sorted(requested), args.sample_count))

    clockwise = parse_pages(args.rotate_cw_pages)
    counterclockwise = parse_pages(args.rotate_ccw_pages)
    upside_down = parse_pages(args.rotate_180_pages)
    overlap = (clockwise & counterclockwise) | (clockwise & upside_down) | (
        counterclockwise & upside_down
    )
    if overlap:
        raise ValueError(f"pages have conflicting rotation overrides: {sorted(overlap)}")
    mixed_orientation = parse_pages(args.mixed_orientation_pages)

    rotations = pdf_rotations(pdf_path, min(requested), max(requested))
    metadata_path = output_dir / "render-metadata.json"
    if metadata_path.is_file():
        payload = json.loads(metadata_path.read_text(encoding="utf-8"))
    else:
        payload = {
            "schema_version": 1,
            "source_pdf": str(pdf_path),
            "format": "PNG",
            "lossless": True,
            "pages": [],
        }
    records = {record["page_number"]: record for record in payload.get("pages", [])}

    requested_pages = sorted(requested)
    targets: dict[int, str] = {}
    for page_number in requested_pages:
        existing = records.get(page_number, {})
        target_name = existing.get("image_filename", f"page-{page_number:04d}.png")
        target = output_dir / target_name
        if target.exists() and not args.overwrite:
            raise FileExistsError(f"{target} exists; pass --overwrite to replace it")
        targets[page_number] = target_name

    with tempfile.TemporaryDirectory(prefix=".render-work-", dir=output_dir) as work:
        work_dir = Path(work)
        with ThreadPoolExecutor(max_workers=args.workers) as executor:
            futures = {}
            for page_number in requested_pages:
                post_rotation = (
                    90
                    if page_number in clockwise
                    else 270
                    if page_number in counterclockwise
                    else 180
                    if page_number in upside_down
                    else 0
                )
                future = executor.submit(
                    render_page,
                    pdf_path=pdf_path,
                    output_dir=output_dir,
                    work_dir=work_dir,
                    page_number=page_number,
                    target_name=targets[page_number],
                    dpi=args.dpi,
                    post_rotation=post_rotation,
                    source_rotation=rotations.get(page_number, 0),
                    orientation_verified=args.orientation_verified,
                    mixed_orientation=page_number in mixed_orientation,
                )
                futures[future] = page_number

            for index, future in enumerate(as_completed(futures), start=1):
                record = future.result()
                page_number = record["page_number"]
                records[page_number] = record
                print(
                    f"Rendered {index}/{len(requested_pages)}: page {page_number}",
                    flush=True,
                )

    payload.update(
        {
            "generated_at_utc": datetime.now(timezone.utc).isoformat(),
            "source_pdf": str(pdf_path),
            "dpi": args.dpi,
            "orientation_policy": "dominant-map-content-upright",
            "selected_pages": sorted(records),
            "pages": [records[number] for number in sorted(records)],
        }
    )
    atomic_json(metadata_path, payload)
    print(f"Render metadata: {metadata_path}")


if __name__ == "__main__":
    main()
