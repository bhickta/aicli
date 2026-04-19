"""Pydantic schemas for the UPSC Analyze API."""
from pydantic import BaseModel, Field
from typing import List, Dict, Any, Optional

class PDFListItemDTO(BaseModel):
    id: int
    filename: str
    page_count: int
    progress: Dict[str, str]

class ProcessingStatusDTO(BaseModel):
    total_pdfs: int
    total_pages: int
    classified_pages: int
    errors: Dict[str, int]

class PageDTO(BaseModel):
    id: int
    page_number: int
    pdf_file: str
    image_path: str
    transcription: Optional[str] = None
    classification: Optional[str] = None
    processed: bool = False

class AnswerDTO(BaseModel):
    id: int
    question_number: Optional[str] = None
    question_directive: Optional[str] = None
    question_text: Optional[str] = None
    raw_text: Optional[str] = None
    page_ids: str  # JSON string in DB, but we keep as is or parse if needed

class AnswerDimensionDTO(BaseModel):
    dimension_name: str
    result_json: Any

class AggregationDTO(BaseModel):
    dimension_name: str
    answer_count: int
    aggregation_json: Any

class RunPipelineRequestDTO(BaseModel):
    workers: int = 4
    dpi: int = 300
    llm_model: str = "gemma-4-26b-a4b"
    allow_reasoning: bool = True
    target_steps: Optional[List[int]] = None
    step_reasoning: Optional[Dict[str, bool]] = None
    page_id: Optional[int] = None

class ResetPipelineRequestDTO(BaseModel):
    step: int = 2

class RetryErrorsResponseDTO(BaseModel):
    ok: bool
    cleared: int
