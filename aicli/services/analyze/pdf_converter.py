"""Step 1: Convert PDF pages to PNG images.

Uses pdf2image (poppler) to render each page at configurable DPI.
Already-converted pages are skipped for resumability.
"""
from pathlib import Path

from pdf2image import convert_from_path

from aicli.domains.analyze.database import AnalyzeDB


class PDFConverterService:
    """Convert PDF pages to PNG images at specified DPI."""

    def convert(self, pdf_path: Path, output_dir: Path, db: AnalyzeDB, dpi: int = 200) -> int:
        """Convert all pages of a PDF to PNG images.

        Args:
            pdf_path: Path to the PDF file.
            output_dir: Directory to store page images.
            db: Database instance.
            dpi: Resolution for rendering. 200 is good for handwriting.

        Returns:
            Count of NEW pages converted (0 if already done).
        """
        pdf_name = pdf_path.name
        existing = db.get_pages_for_pdf(pdf_name)
        if existing:
            return 0  # Already converted

        # Create per-PDF image directory
        pdf_image_dir = output_dir / pdf_path.stem
        pdf_image_dir.mkdir(parents=True, exist_ok=True)

        # Convert all pages at once (pdf2image handles memory internally)
        images = convert_from_path(str(pdf_path), dpi=dpi)

        count = 0
        for i, img in enumerate(images, start=1):
            image_path = pdf_image_dir / f"page_{i:04d}.png"
            img.save(str(image_path), "PNG")
            db.insert_page(pdf_name, i, str(image_path))
            count += 1

        db.log_processing(pdf_name, "pdf_to_images", "done")
        return count

    def convert_all(
        self, data_dir: Path, output_dir: Path, db: AnalyzeDB, dpi: int = 200
    ) -> tuple[int, int]:
        """Convert all PDFs in a directory.

        Returns:
            (pdf_count, total_new_pages)
        """
        pdf_files = sorted(data_dir.glob("*.pdf"))
        total_pages = 0
        pdf_count = 0

        for pdf_path in pdf_files:
            new_pages = self.convert(pdf_path, output_dir, db, dpi)
            if new_pages > 0:
                pdf_count += 1
                total_pages += new_pages

        return pdf_count, total_pages
