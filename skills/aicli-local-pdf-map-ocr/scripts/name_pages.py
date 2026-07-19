#!/usr/bin/env python3
from __future__ import annotations

import argparse
import concurrent.futures
import json
import multiprocessing
import os
import re
from datetime import datetime, timezone
from pathlib import Path


_worker_provider = None


def init_aicli_worker(base_url: str, model_name: str) -> None:
    global _worker_provider
    os.environ["LM_STUDIO_BASE_URL"] = base_url
    os.environ["MODEL_NAME"] = model_name
    from aicli.providers.lm_studio import LMStudioProvider

    _worker_provider = LMStudioProvider()


def naming_prompt(page_number: int, ocr_text: str) -> str:
    return f"""You are naming a high-resolution page image extracted from an NCERT geography/map PDF.

Based ONLY on the OCR below, return one precise kebab-case filename slug.

Rules:
- Output only the slug, with no extension, quotes, markdown, or explanation.
- Use 4 to 8 lowercase words separated by hyphens.
- Prefer the explicit map/figure title or caption.
- Include the principal region, subject, time period, or year when stated.
- If several maps occur, name their shared topic or the dominant map.
- Avoid generic words such as page, image, picture, scan, ncert, textbook, unknown, or unclear.
- Do not invent facts absent from OCR.
- Do not include the PDF page number {page_number}; it is added separately.

OCR:
{ocr_text[:24000]}
"""


def generate_name(task: tuple[int, str]) -> tuple[int, str]:
    page_number, ocr_text = task
    raw = _worker_provider.complete_text(
        naming_prompt(page_number, ocr_text),
        temperature=0.0,
        max_tokens=96,
        max_retries=2,
    )
    return page_number, raw


def sanitize_slug(raw: str) -> str:
    lines = [line.strip().strip("`\"'") for line in raw.splitlines() if line.strip()]
    text = lines[0] if lines else raw.strip()
    text = re.sub(r"^(filename|slug|answer|output)\s*:\s*", "", text, flags=re.I)
    text = text.lower()
    text = re.sub(r"[^a-z0-9]+", "-", text)
    text = re.sub(r"-{2,}", "-", text).strip("-")
    forbidden = {"page", "image", "picture", "scan", "ncert", "textbook"}
    words = [word for word in text.split("-") if word and word not in forbidden]
    words = words[:8]
    if len(words) < 3:
        raise ValueError(f"model returned an unusable filename: {raw!r}")
    return "-".join(words)


def atomic_write_json(path: Path, payload: dict) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    os.replace(temporary, path)


def sync_render_metadata(
    metadata_path: Path, records: dict[int, dict]
) -> int:
    if not metadata_path.is_file():
        return 0

    payload = json.loads(metadata_path.read_text(encoding="utf-8"))
    filenames = {
        page_number: record["image_filename"]
        for page_number, record in records.items()
    }
    updated = 0
    for record in payload.get("pages", []):
        page_number = record.get("page_number")
        filename = filenames.get(page_number)
        if filename is not None and record.get("image_filename") != filename:
            record["image_filename"] = filename
            updated += 1

    payload["filenames_synced_at_utc"] = datetime.now(timezone.utc).isoformat()
    atomic_write_json(metadata_path, payload)
    return updated


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--base-url", default="http://127.0.0.1:1234/v1")
    parser.add_argument("--model", default="local-model")
    parser.add_argument(
        "--model-label", default="lmstudio-community/gemma-4-12B-it-QAT-GGUF"
    )
    parser.add_argument(
        "--sync-render-metadata-only",
        action="store_true",
        help="Synchronize render-metadata.json filenames from the manifest and exit.",
    )
    args = parser.parse_args()

    manifest_path = args.manifest.resolve()
    output_dir = manifest_path.parent
    payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    records = {record["page_number"]: record for record in payload["pages"]}

    render_metadata_path = output_dir / "render-metadata.json"
    if args.sync_render_metadata_only:
        updated = sync_render_metadata(render_metadata_path, records)
        print(f"Render metadata filenames synchronized: {updated}")
        return

    context = multiprocessing.get_context("spawn")
    futures: dict[concurrent.futures.Future, int] = {}
    with concurrent.futures.ProcessPoolExecutor(
        max_workers=args.workers,
        mp_context=context,
        initializer=init_aicli_worker,
        initargs=(args.base_url, args.model),
    ) as pool:
        for page_number, record in records.items():
            future = pool.submit(
                generate_name,
                (page_number, record["ocr"]["combined_text"]),
            )
            futures[future] = page_number

        completed = 0
        for future in concurrent.futures.as_completed(futures):
            page_number = futures[future]
            record = records[page_number]
            try:
                _, raw = future.result()
                slug = sanitize_slug(raw)
                error = None
            except Exception as exc:
                raw = ""
                slug = None
                error = f"{type(exc).__name__}: {exc}"

            record["descriptive_slug"] = slug
            record["naming"] = {
                "model": args.model_label,
                "source": "ocr.combined_text",
                "raw": raw,
                "error": error,
            }
            completed += 1
            if completed % 10 == 0 or completed == len(futures):
                atomic_write_json(manifest_path, payload)
                print(f"Naming progress: {completed}/{len(futures)}", flush=True)

    failures = [
        record["page_number"]
        for record in records.values()
        if record.get("naming", {}).get("error") or not record.get("descriptive_slug")
    ]
    if failures:
        print(f"Naming failures: {failures}")
        raise SystemExit(1)

    moves: list[tuple[Path, Path, dict]] = []
    for page_number in sorted(records):
        record = records[page_number]
        source = output_dir / record["image_filename"]
        target_name = f"page-{page_number:04d}-{record['descriptive_slug']}.png"
        target = output_dir / target_name
        if source != target:
            if not source.is_file():
                raise FileNotFoundError(source)
            if target.exists():
                raise FileExistsError(target)
        moves.append((source, target, record))

    for source, target, record in moves:
        if source != target:
            os.rename(source, target)
        record["image_filename"] = target.name

    payload["naming"] = {
        "generated_at_utc": datetime.now(timezone.utc).isoformat(),
        "local_only": True,
        "model": args.model_label,
        "input": "ocr.combined_text",
    }
    atomic_write_json(manifest_path, payload)
    synchronized = sync_render_metadata(render_metadata_path, records)
    print(f"Renamed images: {len(moves)}")
    print(f"Render metadata filenames synchronized: {synchronized}")


if __name__ == "__main__":
    main()
