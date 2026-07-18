#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import math
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont, ImageOps


def load_font(size: int) -> ImageFont.ImageFont:
    for path in (
        "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
        "/usr/share/fonts/dejavu/DejaVuSans.ttf",
    ):
        if Path(path).is_file():
            return ImageFont.truetype(path, size=size)
    return ImageFont.load_default()


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Build JPEG review sheets without altering final PNGs."
    )
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--per-sheet", type=int, default=25)
    parser.add_argument("--columns", type=int, default=5)
    parser.add_argument("--cell-width", type=int, default=480)
    parser.add_argument("--cell-height", type=int, default=640)
    args = parser.parse_args()

    manifest_path = args.manifest.resolve()
    output_dir = manifest_path.parent
    payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    records = sorted(payload["pages"], key=lambda item: item["page_number"])
    label_font = load_font(16)

    for start in range(0, len(records), args.per_sheet):
        group = records[start : start + args.per_sheet]
        rows = math.ceil(len(group) / args.columns)
        sheet = Image.new(
            "RGB", (args.columns * args.cell_width, rows * args.cell_height), "white"
        )
        draw = ImageDraw.Draw(sheet)
        for position, record in enumerate(group):
            row, column = divmod(position, args.columns)
            left, top = column * args.cell_width, row * args.cell_height
            path = output_dir / record["image_filename"]
            with Image.open(path) as opened:
                image = ImageOps.exif_transpose(opened).convert("RGB")
                image.thumbnail(
                    (args.cell_width - 20, args.cell_height - 60),
                    Image.Resampling.LANCZOS,
                )
                x = left + (args.cell_width - image.width) // 2
                y = top + (args.cell_height - 50 - image.height) // 2
                sheet.paste(image, (x, y))
            label = Path(record["image_filename"]).stem
            text_box = draw.textbbox((0, 0), label, font=label_font)
            text_width = text_box[2] - text_box[0]
            if text_width > args.cell_width - 10:
                keep = max(12, int(len(label) * (args.cell_width - 10) / text_width))
                label = label[:keep]
            draw.text(
                (left + args.cell_width // 2, top + args.cell_height - 34),
                label,
                fill="black",
                font=label_font,
                anchor="mm",
            )

        first = group[0]["page_number"]
        last = group[-1]["page_number"]
        target = output_dir / f"00-contact-sheet-pages-{first:04d}-{last:04d}.jpg"
        sheet.save(target, format="JPEG", quality=92, subsampling=0, optimize=True)
        print(target)


if __name__ == "__main__":
    main()
