"""Constants for the UPSC Analyze pipeline."""
from enum import IntEnum


class PipelineStep(IntEnum):
    """Typed enum for pipeline step IDs — replaces magic integers."""
    PDF_TO_IMAGES = 1
    OCR_TRANSCRIBE = 2
    PAGE_CLASSIFY = 3
    ANSWER_SEGMENT = 4
    DIMENSION_ANALYZE = 5
    CROSS_PDF_AGGREGATE = 6
    REPORT_GEN = 7


# Legacy aliases for backward compatibility
STEP_PDF_TO_IMAGES = PipelineStep.PDF_TO_IMAGES
STEP_OCR_TRANSCRIBE = PipelineStep.OCR_TRANSCRIBE
STEP_PAGE_CLASSIFY = PipelineStep.PAGE_CLASSIFY
STEP_ANSWER_SEGMENT = PipelineStep.ANSWER_SEGMENT
STEP_DIMENSION_ANALYZE = PipelineStep.DIMENSION_ANALYZE
STEP_CROSS_PDF_AGGREGATE = PipelineStep.CROSS_PDF_AGGREGATE
STEP_REPORT_GEN = PipelineStep.REPORT_GEN

STEP_NAMES: dict[int, str] = {
    PipelineStep.PDF_TO_IMAGES: "PDF → Page Images",
    PipelineStep.OCR_TRANSCRIBE: "OCR Transcription",
    PipelineStep.PAGE_CLASSIFY: "Page Classification",
    PipelineStep.ANSWER_SEGMENT: "Answer Segmentation",
    PipelineStep.DIMENSION_ANALYZE: "Dimension Analysis",
    PipelineStep.CROSS_PDF_AGGREGATE: "Cross-PDF Aggregation",
    PipelineStep.REPORT_GEN: "Report Generation",
}

# Default Configuration
DEFAULT_WORKERS: int = 4
DEFAULT_DPI: int = 300
DEFAULT_LLM_MODEL: str = "gemma-4-26b-a4b"

# Error Prefixes
TRANSCRIPTION_ERROR_PREFIX: str = "[TRANSCRIPTION_ERROR:"

# Pipeline Step Reasoning Defaults (Recommended)
RECOMMENDED_REASONING: dict[int, bool] = {
    PipelineStep.PDF_TO_IMAGES: False,
    PipelineStep.OCR_TRANSCRIBE: False,
    PipelineStep.PAGE_CLASSIFY: False,
    PipelineStep.ANSWER_SEGMENT: False,
    PipelineStep.DIMENSION_ANALYZE: True,
    PipelineStep.CROSS_PDF_AGGREGATE: True,
    PipelineStep.REPORT_GEN: True,
}
