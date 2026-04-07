import typer
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn
from rich.table import Table

from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.image_renamer import ImageRenamerService
from aicli.cli.tui import print_header, print_success, print_error, confirm_action, console

app = typer.Typer(help="Image management commands.")

def _fetch_suggestion(img_path: Path, service: ImageRenamerService) -> tuple[Path, str, Exception]:
    """Helper to be run in a separate thread. Just fetches the name."""
    try:
        suggested_name = service.generate_new_name(str(img_path))
        if not suggested_name:
            return img_path, None, ValueError("AI did not return a valid name.")
        return img_path, suggested_name, None
    except Exception as e:
        return img_path, None, e

def _sync_file_references(working_dir: Path, old_name: str, new_name: str):
    """Scans all .md and .json files in the directory and replaces occurrences of old_name with new_name."""
    if not working_dir or not working_dir.is_dir():
        return
        
    extensions = {".md", ".json"}
    for file_path in working_dir.iterdir():
        if file_path.is_file() and file_path.suffix.lower() in extensions:
            try:
                content = file_path.read_text(encoding="utf-8")
                if old_name in content:
                    new_content = content.replace(old_name, new_name)
                    file_path.write_text(new_content, encoding="utf-8")
                    console.print(f"[dim]Synced references in {file_path.name}[/dim]")
            except Exception as e:
                console.print(f"[yellow]Failed to sync refs in {file_path.name}: {e}[/yellow]")

def _apply_rename_safe(img_path: Path, suggested_name: str, service: ImageRenamerService, sync_refs: bool = False, working_dir: Path = None) -> str:
    """Renames file silently and prints error if fails. Returns the new path if successful."""
    try:
        new_path_str = service.apply_rename(str(img_path), suggested_name)
        if new_path_str and sync_refs and working_dir:
            _sync_file_references(working_dir, img_path.name, Path(new_path_str).name)
        return new_path_str
    except Exception as e:
        console.print(f"[red]Failed to rename {img_path.name}: {str(e)}[/red]")
        return ""


def _process_single_image(image_path: Path, service: ImageRenamerService, auto_rename: bool, sync_refs: bool):
    """Processes a single image sequentially."""
    print_header(f"Inspecting {image_path.name}")
    suggested_name = None
    
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        transient=True,
        console=console
    ) as progress:
        progress.add_task(description="Asking LM Studio for a name...", total=None)
        _, suggested_name, err = _fetch_suggestion(image_path, service)
            
    if err:
        print_error(f"Failed to communicate with LM Studio for {image_path.name}.", err)
        return
        
    full_new_name = f"{suggested_name}{image_path.suffix}"
    console.print(f"AI suggested name: [bold yellow]{full_new_name}[/bold yellow]")
    
    if not auto_rename:
        if not confirm_action(f"Do you want to rename [cyan]{image_path.name}[/cyan] to [green]{full_new_name}[/green]?"):
            console.print("[yellow]Rename cancelled by user.[/yellow]\n")
            return

    try:
        new_path = service.apply_rename(str(image_path), suggested_name)
        if sync_refs:
            _sync_file_references(image_path.parent, image_path.name, Path(new_path).name)
        print_success(f"File successfully renamed to: [bold underline]{Path(new_path).name}[/bold underline]\n")
    except Exception as e:
        print_error(f"Failed to rename file {image_path.name}", e)


@app.command("rename")
def rename_image(
    target_path: Path = typer.Argument(
        ...,
        exists=True,
        file_okay=True,
        dir_okay=True,
        help="Path to the image or directory of images you want to rename."
    ),
    auto_rename: bool = typer.Option(
        False, 
        "--auto", "-a", 
        help="Automatically rename without asking for confirmation."
    ),
    workers: int = typer.Option(
        4,
        "--workers", "-w",
        help="Number of concurrent LM inferences when processing a directory."
    ),
    sync_refs: bool = typer.Option(
        False,
        "--sync-refs",
        help="Update references to renamed images inside .md and .json files in the same directory."
    )
):
    """
    Uses LM Studio Vision to scan an image (or a folder of images) and intelligently rename them.
    Can use multiple parallel workers to speed up directory processing.
    """
    try:
        provider = LMStudioProvider()
        service = ImageRenamerService(provider)
    except Exception as e:
        print_error("Failed to initialize AI Provider", e)
        raise typer.Exit(code=1)

    if target_path.is_file():
        _process_single_image(target_path, service, auto_rename, sync_refs)
        raise typer.Exit(code=0)

    # Directory processing via ThreadPool
    console.print(f"[bold cyan]Scanning directory {target_path} for images...[/bold cyan]")
    valid_extensions = {".jpg", ".jpeg", ".png", ".webp", ".gif"}
    images = [p for p in target_path.iterdir() if p.is_file() and p.suffix.lower() in valid_extensions]
    
    if not images:
        console.print("[yellow]No supported images found in the directory.[/yellow]")
        raise typer.Exit()
        
    console.print(f"[bold green]Found {len(images)} images to process using {workers} parallel workers.[/bold green]\n")
    
    # Store results: list of dicts
    results = []

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TaskProgressColumn(),
        console=console
    ) as progress:
        task_id = progress.add_task(f"Inspecting via LM Studio...", total=len(images))
        
        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = [executor.submit(_fetch_suggestion, img, service) for img in images]
            
            for future in as_completed(futures):
                img_path, suggested_name, err = future.result()
                
                if err:
                    progress.console.print(f"[{img_path.name}] [red]API Error: {str(err)}[/red]")
                elif suggested_name:
                    # If auto rename is checked, we rename it immediately right now
                    if auto_rename:
                        new_path = _apply_rename_safe(img_path, suggested_name, service, sync_refs=sync_refs, working_dir=target_path)
                        if new_path:
                            progress.console.print(f"[green]✔ Renamed: {img_path.name} → {Path(new_path).name}[/green]")
                    else:
                        progress.console.print(f"[blue]✨ Evaluated: {img_path.name} → {suggested_name}{img_path.suffix}[/blue]")
                
                results.append((img_path, suggested_name, err))
                progress.advance(task_id)

    # Calculate summaries
    successful = [r for r in results if not r[2]]
    failures = [r for r in results if r[2]]

    if failures:
        console.print(f"[bold red]Failed to process {len(failures)} images based on API errors.[/bold red]")

    # If --auto was enabled, we're basically done!
    if auto_rename:
        print_success(f"Successfully auto-renamed {len(successful)} images!")
        raise typer.Exit(code=0)

    # If --auto is NOT enabled, we must ask the user for confirmation bulk style
    if not successful:
        console.print("[yellow]No valid suggestions generated to be renamed.[/yellow]")
        raise typer.Exit(code=0)

    # Draw table
    table = Table(title="AI Rename Suggestions", show_lines=True)
    table.add_column("Original Filename", style="cyan", no_wrap=True)
    table.add_column("Suggested Rename", style="green")

    for img_path, suggested_name, err in successful:
        table.add_row(img_path.name, f"{suggested_name}{img_path.suffix}")

    console.print(table)
    console.print(f"\n[bold green]Ready to apply {len(successful)} renames![/bold green]")
    
    if confirm_action("Do you want to apply all these renames bulk?"):
        for img_path, suggested_name, _ in successful:
            _apply_rename_safe(img_path, suggested_name, service, sync_refs=sync_refs, working_dir=target_path)
        print_success("Bulk rename complete!")
    else:
        console.print("[yellow]Action cancelled. No files were renamed.[/yellow]")

    raise typer.Exit(code=0)
