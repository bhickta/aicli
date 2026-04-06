import typer
from pathlib import Path
from rich.progress import Progress, SpinnerColumn, TextColumn

from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.image_renamer import ImageRenamerService
from aicli.cli.tui import print_header, print_success, print_error, confirm_action, console

app = typer.Typer(help="Image management commands.")

def _process_single_image(image_path: Path, service: ImageRenamerService, auto_rename: bool):
    """Processes a single image."""
    print_header(f"Inspecting {image_path.name}")
    suggested_name = None
    
    # Use Rich Progress spinner while waiting for LM Studio
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        transient=True,
        console=console
    ) as progress:
        progress.add_task(description="Asking LM Studio for a name...", total=None)
        try:
            suggested_name = service.generate_new_name(str(image_path))
        except Exception as e:
            print_error(f"Failed to communicate with LM Studio for {image_path.name}.", e)
            print_error("Ensure LM Studio Local Server is running on http://localhost:1234/v1")
            return
            
    if not suggested_name:
        print_error(f"AI did not return a valid name for {image_path.name}.")
        return
        
    full_new_name = f"{suggested_name}{image_path.suffix}"
    console.print(f"AI suggested name: [bold yellow]{full_new_name}[/bold yellow]")
    
    # Handle auto-renaming vs confirmation
    if not auto_rename:
        if not confirm_action(f"Do you want to rename [cyan]{image_path.name}[/cyan] to [green]{full_new_name}[/green]?"):
            console.print("[yellow]Rename cancelled by user.[/yellow]\n")
            return

    try:
        new_path = service.apply_rename(str(image_path), suggested_name)
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
    )
):
    """
    Uses LM Studio Vision to scan an image (or a folder of images) and intelligently rename them.
    """
    # Initialize our dependencies
    try:
        provider = LMStudioProvider()
        service = ImageRenamerService(provider)
    except Exception as e:
        print_error("Failed to initialize AI Provider", e)
        raise typer.Exit(code=1)

    if target_path.is_file():
        # Single file processing
        _process_single_image(target_path, service, auto_rename)
    elif target_path.is_dir():
        # Directory processing
        console.print(f"[bold cyan]Scanning directory {target_path} for images...[/bold cyan]")
        valid_extensions = {".jpg", ".jpeg", ".png", ".webp", ".gif"}
        images = [p for p in target_path.iterdir() if p.is_file() and p.suffix.lower() in valid_extensions]
        
        if not images:
            console.print("[yellow]No supported images found in the directory.[/yellow]")
            raise typer.Exit()
            
        console.print(f"[bold green]Found {len(images)} images to process.[/bold green]\n")
        
        for index, img_path in enumerate(images, 1):
            console.print(f"[bold magenta]--- Processing {index}/{len(images)} ---[/bold magenta]")
            _process_single_image(img_path, service, auto_rename)

    raise typer.Exit(code=0)
