from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

from rich.progress import (
    Progress,
    SpinnerColumn,
    TextColumn,
    BarColumn,
    TaskProgressColumn,
)
from rich.table import Table

from aicli.providers.ollama import OllamaProvider
from aicli.services.image_renamer import ImageRenamerService
from aicli.cli.tui import (
    print_header,
    print_success,
    print_error,
    confirm_action,
    console,
)
from aicli.server.services.image_pipeline_service import ImagePipelineService


def _fetch_suggestion(
    img_path: Path, service: ImageRenamerService, trash_junk: bool = False
) -> tuple[Path, str, Exception]:
    try:
        suggested_name = service.generate_new_name(str(img_path), trash_junk=trash_junk)
        if not suggested_name:
            return img_path, None, ValueError("AI did not return a valid name.")
        return img_path, suggested_name, None
    except Exception as e:
        return img_path, None, e


def _apply_rename_safe(
    img_path: Path,
    suggested_name: str,
    service: ImageRenamerService,
    sync_refs: bool = False,
    working_dir: Path = None,
) -> str:
    try:
        new_path_str = service.apply_rename(str(img_path), suggested_name)
        if new_path_str and sync_refs and working_dir:
            ImagePipelineService.sync_file_references(
                working_dir, img_path.name, Path(new_path_str).name
            )
        return new_path_str
    except Exception as e:
        console.print(f"[red]Failed to rename {img_path.name}: {str(e)}[/red]")
        return ""


def _process_single_image(
    image_path: Path,
    service: ImageRenamerService,
    auto_rename: bool,
    sync_refs: bool,
    trash_junk: bool,
):
    print_header(f"Inspecting {image_path.name}")
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        transient=True,
        console=console,
    ) as progress:
        progress.add_task(description="Asking LM Studio for a name...", total=None)
        _, suggested_name, err = _fetch_suggestion(
            image_path, service, trash_junk=trash_junk
        )

    if err:
        print_error(f"Failed to communicate with LM Studio for {image_path.name}.", err)
        return

    if suggested_name == "TRASH":
        console.print(
            f"[bold dark_orange]AI flagged {image_path.name} as JUNK.[/bold dark_orange]"
        )
        if not auto_rename and not confirm_action(
            f"Trash [cyan]{image_path.name}[/cyan] into `.trash`?"
        ):
            return
        ImagePipelineService.apply_trash_safe(
            image_path, sync_refs=sync_refs, working_dir=image_path.parent
        )
        print_success("File moved to .trash!\n")
        return

    full_new_name = f"{suggested_name}{image_path.suffix}"
    console.print(f"AI suggested name: [bold yellow]{full_new_name}[/bold yellow]")

    if not auto_rename and not confirm_action(
        f"Rename [cyan]{image_path.name}[/cyan] to [green]{full_new_name}[/green]?"
    ):
        return

    try:
        new_path = service.apply_rename(str(image_path), suggested_name)
        if sync_refs:
            ImagePipelineService.sync_file_references(
                image_path.parent, image_path.name, Path(new_path).name
            )
        print_success(
            f"File successfully renamed to: [bold underline]{Path(new_path).name}[/bold underline]\n"
        )
    except Exception as e:
        print_error(f"Failed to rename file {image_path.name}", e)


def rename_image(
    target_path: Path,
    auto_rename: bool = False,
    workers: int = 4,
    sync_refs: bool = False,
    trash_junk: bool = False,
):
    try:
        service = ImageRenamerService(OllamaProvider())
    except Exception as e:
        return print_error("Failed to initialize AI Provider", e)

    if target_path.is_file():
        return _process_single_image(
            target_path, service, auto_rename, sync_refs, trash_junk
        )

    console.print(
        f"[bold cyan]Scanning directory {target_path} for images...[/bold cyan]"
    )
    images = [
        p
        for p in target_path.iterdir()
        if p.is_file()
        and p.suffix.lower() in {".jpg", ".jpeg", ".png", ".webp", ".gif"}
    ]
    if not images:
        return console.print(
            "[yellow]No supported images found in the directory.[/yellow]"
        )

    console.print(
        f"[bold green]Found {len(images)} images to process using {workers} parallel workers.[/bold green]\n"
    )
    results = []

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TaskProgressColumn(),
        console=console,
    ) as progress:
        task_id = progress.add_task(f"Inspecting via LM Studio...", total=len(images))
        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = [
                executor.submit(_fetch_suggestion, img, service, trash_junk)
                for img in images
            ]
            for future in as_completed(futures):
                img_path, suggested_name, err = future.result()
                if err:
                    progress.console.print(
                        f"[{img_path.name}] [red]API Error: {str(err)}[/red]"
                    )
                elif suggested_name == "TRASH":
                    if auto_rename:
                        ImagePipelineService.apply_trash_safe(
                            img_path, sync_refs=sync_refs, working_dir=target_path
                        )
                        progress.console.print(
                            f"[dark_orange]🗑 Moved to trash: {img_path.name}[/dark_orange]"
                        )
                    else:
                        progress.console.print(
                            f"[dark_orange]🏷 Flagged as junk: {img_path.name}[/dark_orange]"
                        )
                elif suggested_name:
                    if auto_rename:
                        new_path = _apply_rename_safe(
                            img_path,
                            suggested_name,
                            service,
                            sync_refs=sync_refs,
                            working_dir=target_path,
                        )
                        if new_path:
                            progress.console.print(
                                f"[green]✔ Renamed: {img_path.name} → {Path(new_path).name}[/green]"
                            )
                    else:
                        progress.console.print(
                            f"[blue]✨ Evaluated: {img_path.name} → {suggested_name}{img_path.suffix}[/blue]"
                        )

                results.append((img_path, suggested_name, err))
                progress.advance(task_id)

    successful = [r for r in results if not r[2]]
    failures = [r for r in results if r[2]]

    if failures:
        console.print(
            f"[bold red]Failed to process {len(failures)} images based on API errors.[/bold red]"
        )
    if auto_rename:
        return print_success(f"Successfully auto-renamed {len(successful)} images!")
    if not successful:
        return console.print(
            "[yellow]No valid suggestions generated to be renamed.[/yellow]"
        )

    table = Table(title="AI Rename Suggestions", show_lines=True)
    table.add_column("Original Filename", style="cyan", no_wrap=True)
    table.add_column("Suggested Rename", style="green")
    for img_path, suggested_name, err in successful:
        table.add_row(img_path.name, f"{suggested_name}{img_path.suffix}")

    console.print(table)
    if confirm_action("Do you want to apply all these renames bulk?"):
        for img_path, suggested_name, _ in successful:
            if suggested_name == "TRASH":
                ImagePipelineService.apply_trash_safe(
                    img_path, sync_refs=sync_refs, working_dir=target_path
                )
            else:
                _apply_rename_safe(
                    img_path,
                    suggested_name,
                    service,
                    sync_refs=sync_refs,
                    working_dir=target_path,
                )
        print_success("Bulk operation complete!")


def _fetch_junk_status(
    img_path: Path, service: ImageRenamerService, strict: bool = False
) -> tuple[Path, bool, Exception]:
    try:
        return img_path, service.identify_junk(str(img_path), strict=strict), None
    except Exception as e:
        return img_path, False, e


def clean_images(
    target_path: Path,
    auto_trash: bool = False,
    strict: bool = False,
    sync_refs: bool = False,
    workers: int = 4,
):
    try:
        service = ImageRenamerService(OllamaProvider())
    except Exception as e:
        return print_error("Failed to initialize AI Provider", e)

    console.print(
        f"[bold cyan]Scanning directory {target_path} for junk images...[/bold cyan]"
    )
    images = [
        p
        for p in target_path.iterdir()
        if p.is_file()
        and p.suffix.lower() in {".jpg", ".jpeg", ".png", ".webp", ".gif"}
    ]
    if not images:
        return console.print("[yellow]No supported images found.[/yellow]")

    results = []
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TaskProgressColumn(),
        console=console,
    ) as progress:
        task_id = progress.add_task("Inspecting via LM Studio...", total=len(images))
        with ThreadPoolExecutor(max_workers=workers) as executor:
            for future in as_completed(
                [
                    executor.submit(_fetch_junk_status, img, service, strict)
                    for img in images
                ]
            ):
                img_path, is_junk, err = future.result()
                if err:
                    progress.console.print(
                        f"[{img_path.name}] [red]API Error: {str(err)}[/red]"
                    )
                elif is_junk:
                    if auto_trash:
                        ImagePipelineService.apply_trash_safe(
                            img_path, sync_refs=sync_refs, working_dir=target_path
                        )
                        progress.console.print(
                            f"[dark_orange]🗑 Moved to trash: {img_path.name}[/dark_orange]"
                        )
                    else:
                        progress.console.print(
                            f"[dark_orange]🏷 Flagged as junk: {img_path.name}[/dark_orange]"
                        )
                else:
                    progress.console.print(f"[dim]✓ Keep: {img_path.name}[/dim]")
                results.append((img_path, is_junk, err))
                progress.advance(task_id)

    junk_items = [r for r in results if r[1] and not r[2]]
    if not junk_items:
        return print_success("No junk images detected in this directory!")
    if auto_trash:
        return print_success(f"Successfully cleaned up {len(junk_items)} junk images!")

    console.print(
        f"\n[bold yellow]Found {len(junk_items)} purely cosmetic/useless images.[/bold yellow]"
    )
    if confirm_action("Move all flagged images to .trash bulk?"):
        for img_path, _, _ in junk_items:
            ImagePipelineService.apply_trash_safe(
                img_path, sync_refs=sync_refs, working_dir=target_path
            )
        print_success("Bulk wipe complete!")


def _fetch_ocr_text(
    img_path: Path, service: ImageRenamerService
) -> tuple[Path, str, Exception]:
    try:
        return img_path, service.convert_to_markdown(str(img_path)), None
    except Exception as e:
        return img_path, "", e


def digitize_images(
    target_path: Path,
    auto_replace: bool = False,
    sync_refs: bool = False,
    workers: int = 2,
):
    try:
        service = ImageRenamerService(OllamaProvider())
    except Exception as e:
        return print_error("Failed to initialize AI Provider", e)

    console.print(
        f"[bold cyan]Scanning directory {target_path} for text images...[/bold cyan]"
    )
    images = [
        p
        for p in target_path.iterdir()
        if p.is_file()
        and p.suffix.lower() in {".jpg", ".jpeg", ".png", ".webp", ".gif"}
    ]
    if not images:
        return console.print("[yellow]No supported images found.[/yellow]")

    results = []
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TaskProgressColumn(),
        console=console,
    ) as progress:
        task_id = progress.add_task("Inspecting via LM Studio...", total=len(images))
        with ThreadPoolExecutor(max_workers=workers) as executor:
            for future in as_completed(
                [executor.submit(_fetch_ocr_text, img, service) for img in images]
            ):
                img_path, raw_text, err = future.result()
                is_text = False
                clean_markdown = ""

                if err:
                    progress.console.print(
                        f"[{img_path.name}] [red]API Error: {str(err)}[/red]"
                    )
                elif raw_text == "KEEP":
                    progress.console.print(
                        f"[dim]✓ Keep as Graphic: {img_path.name}[/dim]"
                    )
                else:
                    clean_markdown = raw_text
                    if clean_markdown.upper().startswith("TEXT:"):
                        clean_markdown = clean_markdown[5:].strip()
                        is_text = True
                    elif len(clean_markdown) > 10:
                        is_text = True

                    if is_text:
                        if auto_replace:
                            ImagePipelineService.apply_digitize_safe(
                                img_path,
                                clean_markdown,
                                sync_refs=sync_refs,
                                working_dir=target_path,
                            )
                            progress.console.print(
                                f"[bold magenta]✍ Injected markdown for: {img_path.name}[/bold magenta]"
                            )
                        else:
                            progress.console.print(
                                f"[bold magenta]📝 Scanned markdown for: {img_path.name} ({len(clean_markdown)} chars)[/bold magenta]"
                            )

                results.append((img_path, is_text, clean_markdown, err))
                progress.advance(task_id)

    ocr_items = [r for r in results if r[1] and not r[3]]
    if not ocr_items:
        return print_success("No pure text images suitable for conversion detected!")
    if auto_replace:
        return print_success(
            f"Successfully digitized and replaced {len(ocr_items)} images!"
        )

    console.print(
        f"\n[bold yellow]Found {len(ocr_items)} text-heavy images capable of being fully digitized.[/bold yellow]"
    )
    if confirm_action("Do you want to convert these to text and delete the images?"):
        for img_path, _, markdown, _ in ocr_items:
            ImagePipelineService.apply_digitize_safe(
                img_path, markdown, sync_refs=sync_refs, working_dir=target_path
            )
        print_success("Bulk conversion complete!")


def prune_refs(target_path: Path):
    ImagePipelineService.prune_refs(target_path)
