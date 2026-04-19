import re
import shutil
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn
from rich.table import Table

from aicli.providers.lm_studio import LMStudioProvider
from aicli.services.image_renamer import ImageRenamerService
from aicli.cli.tui import print_header, print_success, print_error, confirm_action, console


class ImagePipelineService:
    @staticmethod
    def sync_file_references(working_dir: Path, old_name: str, new_name: str):
        if not working_dir or not working_dir.is_dir(): return
        for file_path in working_dir.iterdir():
            if file_path.is_file() and file_path.suffix.lower() in {".md", ".json"}:
                try:
                    content = file_path.read_text(encoding="utf-8")
                    if old_name in content:
                        file_path.write_text(content.replace(old_name, new_name), encoding="utf-8")
                        console.print(f"[dim]Synced references in {file_path.name}[/dim]")
                except Exception as e:
                    console.print(f"[yellow]Failed to sync refs in {file_path.name}: {e}[/yellow]")

    @staticmethod
    def apply_trash_safe(img_path: Path, sync_refs: bool = False, working_dir: Path = None) -> str:
        try:
            trash_dir = img_path.parent / ".trash"
            trash_dir.mkdir(exist_ok=True)
            new_path = trash_dir / img_path.name
            
            counter = 1
            while new_path.exists():
                new_path = trash_dir / f"{img_path.stem}-{counter}{img_path.suffix}"
                counter += 1
                
            shutil.move(str(img_path), str(new_path))
            
            if sync_refs and working_dir:
                for file_path in working_dir.iterdir():
                    if file_path.is_file() and file_path.suffix.lower() in {".md", ".json"}:
                        try:
                            content = file_path.read_text(encoding="utf-8")
                            if img_path.name in content:
                                new_content = re.sub(rf'!\[.*?\]\({re.escape(img_path.name)}\)\n?', '', content)
                                new_content = re.sub(rf'<<{re.escape(img_path.name)}>>\n?', '', new_content)
                                new_content = new_content.replace(img_path.name, "")
                                file_path.write_text(new_content, encoding="utf-8")
                                console.print(f"[dim]Removed trash references from {file_path.name}[/dim]")
                        except Exception:
                            pass
            return str(new_path)
        except Exception as e:
            console.print(f"[red]Failed to move {img_path.name} to trash: {str(e)}[/red]")
            return ""

    @staticmethod
    def apply_digitize_safe(img_path: Path, markdown_text: str, sync_refs: bool = False, working_dir: Path = None) -> str:
        try:
            trash_dir = img_path.parent / ".trash" / "converted"
            trash_dir.mkdir(parents=True, exist_ok=True)
            new_path = trash_dir / img_path.name
            
            counter = 1
            while new_path.exists():
                new_path = trash_dir / f"{img_path.stem}-{counter}{img_path.suffix}"
                counter += 1
                
            shutil.move(str(img_path), str(new_path))
            
            if sync_refs and working_dir:
                for file_path in working_dir.iterdir():
                    if file_path.is_file() and file_path.suffix.lower() in {".md", ".json"}:
                        try:
                            content = file_path.read_text(encoding="utf-8")
                            if img_path.name in content:
                                new_content = re.sub(rf'!\[.*?\]\({re.escape(img_path.name)}\)\n?', f"\n{markdown_text}\n\n", content)
                                new_content = re.sub(rf'<<{re.escape(img_path.name)}>>\n?', f"\n{markdown_text}\n\n", new_content)
                                if img_path.name in new_content:
                                    new_content = new_content.replace(img_path.name, markdown_text)
                                file_path.write_text(new_content, encoding="utf-8")
                                console.print(f"[dim]Injected Markdown into {file_path.name}[/dim]")
                        except Exception:
                            pass
            return str(new_path)
        except Exception as e:
            console.print(f"[red]Failed to digitize {img_path.name}: {str(e)}[/red]")
            return ""

    @staticmethod
    def prune_refs(target_path: Path):
        console.print(f"[bold cyan]Scanning directory {target_path} for broken links...[/bold cyan]")
        target_files = [p for p in target_path.iterdir() if p.is_file() and p.suffix.lower() in {".md", ".json"}]
        
        if not target_files:
            console.print("[yellow]No .md or .json files found in the directory.[/yellow]")
            return
            
        broken_links_removed = 0
        modified_files = 0
        pattern = re.compile(r'!\[.*?\]\(([^)]+\.(?:jpg|jpeg|png|webp|gif|svg))\)|<<([^>]+\.(?:jpg|jpeg|png|webp|gif|svg))>>', re.IGNORECASE)
        
        with Progress(SpinnerColumn(), TextColumn("[progress.description]{task.description}"), console=console) as progress:
            progress.add_task("Sweeping for broken references...", total=None)
            for file_path in target_files:
                try:
                    original_content = file_path.read_text(encoding="utf-8")
                    
                    def replacer(match):
                        nonlocal broken_links_removed
                        img_name = match.group(1) or match.group(2)
                        if not img_name: return match.group(0)
                        if not (target_path / img_name.strip()).exists():
                            broken_links_removed += 1
                            return ""
                        return match.group(0)
                        
                    new_content = pattern.sub(replacer, original_content)
                    if new_content != original_content:
                        file_path.write_text(new_content, encoding="utf-8")
                        modified_files += 1
                        console.print(f"[dim]Cleaned broken links in {file_path.name}[/dim]")
                except Exception as e:
                    console.print(f"[red]Failed to process {file_path.name}: {e}[/red]")
                
        print_success(f"Removed {broken_links_removed} broken references across {modified_files} files!")
