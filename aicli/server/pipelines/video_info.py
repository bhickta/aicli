"""Video info/restore commands — View and restore metadata."""
import typer
from pathlib import Path

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn, TimeElapsedColumn, TimeRemainingColumn
from rich.table import Table

from aicli.services.video import FFprobeClient, MetadataBackupManager, VideoTaggerService
from aicli.cli.tui import print_header, print_success, console


def register(app: typer.Typer):
    """Register restore and info commands on the given Typer app."""

    @app.command("restore")
    def restore_tags(
        target_path: Path = typer.Argument(
            ...,
            exists=True,
            help="Path to the video file or directory to restore."
        )
    ):
        """
        Restore original tags from the backup sidecar (undoing any --write actions).
        """
        valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS
        if target_path.is_file():
            files = [target_path] if target_path.suffix.lower() in valid_extensions else []
        else:
            files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

        if not files:
            console.print("[yellow]No supported video files found.[/yellow]")
            raise typer.Exit()
            
        print_header(f"Restoring {len(files)} files")
        
        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            BarColumn(),
            TaskProgressColumn(),
            TimeElapsedColumn(),
            TextColumn("•"),
            TimeRemainingColumn(),
            console=console
        ) as progress:
            task_id = progress.add_task("Restoring backup sidecars...", total=len(files))
            for f in files:
                try:
                    if MetadataBackupManager.restore_original_tags(f):
                        progress.console.print(f"[green]✔ Restored {f.name}[/green]")
                    else:
                        progress.console.print(f"[yellow]⚠ No backup tags to restore in {f.name}[/yellow]")
                except Exception as e:
                    progress.console.print(f"[red]✖ Failed to restore {f.name}: {e}[/red]")
                progress.advance(task_id)

        print_success("Restore operation complete.")

    @app.command("info")
    def view_metadata(
        target_path: Path = typer.Argument(
            ...,
            exists=True,
            help="Path to the video file or directory."
        )
    ):
        """
        View current metadata of a video file.
        """
        valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS
        if target_path.is_file():
            files = [target_path] if target_path.suffix.lower() in valid_extensions else []
        else:
            files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

        if not files:
            console.print("[yellow]No supported video files found.[/yellow]")
            raise typer.Exit()
            
        for f in files:
            tags = FFprobeClient.read_existing_tags(f)
            if not tags:
                console.print(f"\n[cyan]{f.name}[/cyan]\n[yellow]No metadata found.[/yellow]")
                continue
                
            table = Table(title=f"Metadata: {f.name}", show_header=True, header_style="bold magenta")
            table.add_column("Property", style="dim", width=20)
            table.add_column("Value")
            
            for k, v in tags.items():
                if k.lower() == "aicli_backup":
                    table.add_row(k, "[dim italic]<Embedded Backup Payload>[/dim italic]")
                elif isinstance(v, list):
                    table.add_row(k, ", ".join(map(str, v)))
                else:
                    table.add_row(k, str(v))
                    
            console.print(table)
            console.print()

    @app.command("restore-names")
    def restore_names(
        target_path: Path = typer.Argument(
            ...,
            exists=True,
            help="Path to the video file or directory."
        )
    ):
        """
        Restore original filenames that were changed by AI renaming.
        Reads the saved original_filename from the .aicli_cache sidecar.
        """
        import shutil
        
        valid_extensions = VideoTaggerService.VIDEO_EXTENSIONS
        if target_path.is_file():
            files = [target_path] if target_path.suffix.lower() in valid_extensions else []
        else:
            files = [p for p in target_path.rglob("*") if p.is_file() and p.suffix.lower() in valid_extensions]

        if not files:
            console.print("[yellow]No supported video files found.[/yellow]")
            raise typer.Exit()
            
        print_header(f"Restoring original filenames for {len(files)} files")
        
        restored = 0
        skipped = 0
        for f in files:
            cache = MetadataBackupManager.load_cache(f)
            original_name = cache.get("original_filename")
            if not original_name:
                console.print(f"[dim]\\[{f.name}] No original filename saved, skipping.[/dim]")
                skipped += 1
                continue

            original_path = f.parent / original_name
            if original_path == f:
                console.print(f"[dim]\\[{f.name}] Already has original name.[/dim]")
                skipped += 1
                continue

            if original_path.exists():
                console.print(f"[yellow]\\[{f.name}] Cannot restore — '{original_name}' already exists.[/yellow]")
                skipped += 1
                continue
            
            shutil.move(str(f), str(original_path))
            MetadataBackupManager.rename_cache_files(f, original_path)
            console.print(f"[green]✔ {f.name} → {original_name}[/green]")
            restored += 1

        print_success(f"Restored {restored} filename(s). Skipped {skipped}.")

