"""SQLite database manager for the UPSC analyze pipeline.

Decomposed into multi-inheritance mixins for clean architecture.
"""
from aicli.domains.analyze.db.base import BaseSQLite
from aicli.domains.analyze.db.pages import PageMixin
from aicli.domains.analyze.db.answers import AnswerMixin
from aicli.domains.analyze.db.dimensions import DimensionMixin
from aicli.domains.analyze.db.logs import LogMixin

class AnalyzeDB(BaseSQLite, PageMixin, AnswerMixin, DimensionMixin, LogMixin):
    """Thread-safe SQLite wrapper for the full analyze pipeline.
    
    Inherits all data access methods from specialized domain mixins:
    - PageMixin: PDF scanning and OCR status
    - AnswerMixin: Segmented answers and mapping
    - DimensionMixin: AI evaluation reasoning
    - LogMixin: Progress tracking and reset logic
    """
    pass
