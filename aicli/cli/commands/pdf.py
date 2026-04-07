import subprocess
import typer
from pathlib import Path
from aicli.cli.tui import print_header, print_success, print_error, console

app = typer.Typer(help="PDF extraction and AI processing pipelines.")

def run_step(command: list[str], description: str):
    console.print(f"\n[bold magenta]==== {description} ====[/bold magenta]")
    # We use subprocess here to easily cascade Typer commands and catch exit codes
    result = subprocess.run(command)
    if result.returncode != 0:
        console.print(f"[bold red]Failed during: {description}[/bold red]")
        raise typer.Exit(code=1)

@app.command("process")
def process_pdf(
    pdf_path: Path = typer.Argument(
        ...,
        exists=True,
        file_okay=True,
        dir_okay=False,
        help="Path to the PDF file."
    ),
    output_dir: Path = typer.Option(
        ...,
        "--output-dir", "-o",
        help="Directory to save the extracted folder.",
        prompt=True
    ),
    workers: int = typer.Option(
        4,
        "--workers", "-w",
        help="Number of concurrent LM inferences for AI tasks."
    ),
    digitize: bool = typer.Option(
        False,
        "--digitize",
        help="Enable Phase 2: Lossless OCR to convert text-heavy images into Markdown. (Disabled by default)"
    )
):
    """
    God-Mode: Extracts a PDF using Marker, then runs deep-clean, OCR digitization, image renaming, and link pruning all in a single pipeline.
    """
    print_header(f"Starting God-Mode Pipeline for {pdf_path.name}")
    
    target_dir = output_dir / pdf_path.stem
    
    # 1. Marker Extraction
    if target_dir.exists() and target_dir.is_dir():
        console.print(f"[bold yellow]Phase 0 Skipped:[/bold yellow] Output directory [cyan]{target_dir}[/cyan] already exists! Skipping 2-minute marker extraction.")
    else:
        cmd_marker = ["marker_single", str(pdf_path), "--output_dir", str(output_dir)]
        run_step(cmd_marker, "Phase 0: PDF Extraction (marker_single)")
        
        if not target_dir.exists() or not target_dir.is_dir():
            print_error("Extraction Failed", ValueError(f"Expected output directory {target_dir} was not created."))
            raise typer.Exit(code=1)
        
    # 2. Deep Clean
    cmd_clean = ["aicli", "image", "clean", str(target_dir), "--auto", "--strict", "--sync-refs", "--workers", str(workers)]
    run_step(cmd_clean, "Phase 1: Deep Clean (Trash Junk)")
    
    # 3. Digitize
    if digitize:
        # Use max 2 workers for digitize to prevent VRAM explosion due to massive OCR context
        dig_workers = min(workers, 2)
        cmd_digitize = ["aicli", "image", "digitize", str(target_dir), "--auto", "--sync-refs", "--workers", str(dig_workers)]
        run_step(cmd_digitize, "Phase 2: Digitize (Lossless OCR)")
    else:
        console.print("\n[bold yellow]==== Phase 2 Skipped: Digitize (OCR disabled by user) ====[/bold yellow]")
    
    # 4. Smart Rename
    cmd_rename = ["aicli", "image", "rename", str(target_dir), "--auto", "--sync-refs", "--workers", str(workers)]
    run_step(cmd_rename, "Phase 3: Smart Object Renaming")
    
    # 5. Prune Refs
    cmd_prune = ["aicli", "image", "prune-refs", str(target_dir)]
    run_step(cmd_prune, "Phase 4: Dead-Link Pruning")
    
    console.print("\n")
    print_success(f"Pipeline Fully Completed! Your amazing notes are ready at: {target_dir}")
