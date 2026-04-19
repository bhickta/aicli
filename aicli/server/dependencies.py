"""Dependencies for the Analyze domain."""
from pathlib import Path
from fastapi import Depends
from aicli.server.repositories.analyze_repository import AnalyzeRepository
from aicli.server.services.analyze_pipeline_service import AnalyzePipelineService
from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.analyze.config_loader import AnalyzeConfig

# This mimics the ServerState but should be configured on app startup
class AnalyzeSettings:
    data_dir = Path("data")
    cache_dir = Path(".analyze_cache/images")

def get_analyze_repository() -> AnalyzeRepository:
    """Dependency provider for AnalyzeRepository."""
    db_path = AnalyzeSettings.data_dir / "analyze.db"
    return AnalyzeRepository(db_path)

def get_analyze_service(
    repo: AnalyzeRepository = Depends(get_analyze_repository)
) -> AnalyzePipelineService:
    """Dependency provider for AnalyzePipelineService."""
    provider = LMStudioProvider()
    config = AnalyzeConfig()
    return AnalyzePipelineService(repo, provider, config)
