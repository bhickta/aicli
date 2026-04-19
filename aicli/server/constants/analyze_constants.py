"""Constants for the UPSC Analyze pipeline."""

# Step Definitions
STEP_PDF_TO_IMAGES = 1
STEP_OCR_TRANSCRIBE = 2
STEP_PAGE_CLASSIFY = 3
STEP_ANSWER_SEGMENT = 4
STEP_DIMENSION_ANALYZE = 5
STEP_CROSS_PDF_AGGREGATE = 6
STEP_REPORT_GEN = 7

STEP_NAMES = {
    STEP_PDF_TO_IMAGES: "PDF → Page Images",
    STEP_OCR_TRANSCRIBE: "OCR Transcription",
    STEP_PAGE_CLASSIFY: "Page Classification",
    STEP_ANSWER_SEGMENT: "Answer Segmentation",
    STEP_DIMENSION_ANALYZE: "Dimension Analysis",
    STEP_CROSS_PDF_AGGREGATE: "Cross-PDF Aggregation",
    STEP_REPORT_GEN: "Report Generation",
}

# Default Configuration
DEFAULT_WORKERS = 4
DEFAULT_DPI = 300
DEFAULT_LLM_MODEL = "gemma-4-26b-a4b"

# Error Prefixes
TRANSCRIPTION_ERROR_PREFIX = "[TRANSCRIPTION_ERROR:"

# Pipeline Step Reasoning Defaults (Recommended)
RECOMMENDED_REASONING = {
    STEP_PDF_TO_IMAGES: False,
    STEP_OCR_TRANSCRIBE: False,
    STEP_PAGE_CLASSIFY: False,
    STEP_ANSWER_SEGMENT: False,
    STEP_DIMENSION_ANALYZE: True,
    STEP_CROSS_PDF_AGGREGATE: True,
    STEP_REPORT_GEN: True,
}
