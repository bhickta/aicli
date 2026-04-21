"""Video course command — GOD-MODE pipeline for full course archival."""

from pathlib import Path
import typer

from aicli.cli.tui import print_header, console
from aicli.server.repositories.video_cache_repository import VideoCacheRepository
from aicli.server.services.video_orchestrator_service import VideoOrchestratorService
from aicli.config import config


def process_course(
    target_dir: Path,
    whisper_model: str = "large-v3",
    cleanup: str = "keep",
    w1: int = 2,
    w2: int = 12,
    w3: int = 12,
    llm_model: str = None,
    max_merge_hours: float = 0.0,
    notes_llm: str = None,
):
    console.print(
        "[bold magenta]===== GOD-MODE COURSE PIPELINE INITIATED =====[/bold magenta]"
    )

    # 1. Preflight Evaluation
    raw_files = VideoCacheRepository.get_raw_files(target_dir)
    if not raw_files:
        console.print("[red]No raw videos found![/red]")
        return

    state = VideoCacheRepository.evaluate_pipeline_state(raw_files, target_dir)
    VideoCacheRepository.print_preflight_dashboard(state, target_dir)

    # 0. Preflight Purge (Nuclear)
    VideoOrchestratorService.run_phase0_preflight_purge()

    # 2. Phase 1: Transcribe
    print_header(f"Phase 1: Transcribe {len(raw_files)} files")
    VideoOrchestratorService.run_phase1_transcribe(
        state["needs_transcription"], whisper_model, w1
    )

    # 3. Phase 2: Tag and Sort
    print_header(f"Phase 2: Intelligent Tagging & Renaming ({w2} workers)")
    renamed_files = VideoOrchestratorService.run_phase2_tag_and_sort(
        raw_files, llm_model, w2
    )

    # 4. Phase 3: Compress
    print_header(f"Phase 3: Slideshow Compression ({w3} workers)")
    slideshow_files = VideoOrchestratorService.run_phase3_compress(renamed_files, w3)

    # 5. Phase 4: Merge
    VideoOrchestratorService.run_phase4_merge(
        target_dir, slideshow_files, max_merge_hours
    )

    # 6. Phase 6: Cleanup
    if cleanup == "trash":
        print_header("Phase 6: Cleaning up intermediate files")
        VideoOrchestratorService.run_phase6_cleanup(target_dir, slideshow_files)

    # Final Teardown
    from aicli.providers import get_provider
    p_name = get_provider().__class__.__name__.replace("Provider", "")
    console.print(f"[cyan]Done with {p_name} inference...[/cyan]")

    console.print(
        "\n[bold magenta]===== GOD-MODE PIPELINE COMPLETE =====[/bold magenta]"
    )
