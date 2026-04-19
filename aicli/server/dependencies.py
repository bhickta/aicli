"""Dependencies and shared state for the AICLI server."""

from pathlib import Path
from fastapi import Depends
from aicli.server.repositories.analyze_repository import AnalyzeRepository
from aicli.server.services.analyze_pipeline_service import AnalyzePipelineService
from aicli.providers import get_provider
from aicli.core.interfaces import ImageVisionProvider
from aicli.services.analyze.config_loader import AnalyzeConfig


class ServerState:
    """Shared server state for directory paths.
    Note: Initialized by the CLI run_server command.
    """

    data_dir: Path = Path("data")
    cache_dir: Path = Path(".analyze_cache/images")


def get_analyze_repository() -> AnalyzeRepository:
    """Dependency provider for AnalyzeRepository."""
    db_path = ServerState.data_dir / "analyze.db"
    return AnalyzeRepository(db_path)


def get_provider_instance() -> ImageVisionProvider:
    """Dependency provider for the configured AI provider."""
    return get_provider()


def get_analyze_service(
    repo: AnalyzeRepository = Depends(get_analyze_repository),
    provider: ImageVisionProvider = Depends(get_provider_instance),
) -> AnalyzePipelineService:
    """Dependency provider for AnalyzePipelineService."""
    config = AnalyzeConfig()
    return AnalyzePipelineService(repo, provider, config)
