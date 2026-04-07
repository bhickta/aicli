import typer
from pathlib import Path
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, MofNCompleteColumn
from aicli.cli.tui import print_header, print_success, print_error, console

app = typer.Typer(help="Current affairs news parsing → filterable Excel.")


@app.command("parse")
def parse_news(
    file_path: Path = typer.Argument(
        ...,
        exists=True,
        file_okay=True,
        dir_okay=False,
        help="Path to the current affairs text/markdown file.",
    ),
    output: Path = typer.Option(
        None,
        "--output", "-o",
        help="Output Excel file path. Defaults to <input_name>_parsed.xlsx. If file exists, new rows are APPENDED.",
    ),
    batch_size: int = typer.Option(
        10,
        "--batch-size", "-b",
        help="Number of lines to send per LLM call (lower = more accurate, higher = faster).",
    ),
    show_prompt: bool = typer.Option(
        False,
        "--show-prompt",
        help="Print the full system prompt to stdout and exit.",
    ),
):
    """
    Parse a current affairs file into a filterable Excel spreadsheet.

    Each line in the file is treated as one news item. The local LLM classifies
    each into Month, Year, Topic, Tags and a clean summary.

    Supports APPEND mode: run on multiple files with the same --output and all
    rows accumulate into a single master Excel sheet.
    """
    from openai import OpenAI
    from aicli.config import config
    from aicli.services.news_parser import (
        build_system_prompt,
        build_user_prompt,
        extract_json_array,
        filter_news_lines,
        write_excel,
        STANDARD_TOPICS,
    )

    system_prompt = build_system_prompt()

    # ── Show prompt mode ──────────────────────────────────────────────
    if show_prompt:
        console.print(system_prompt)
        raise typer.Exit()

    print_header(f"Parsing Current Affairs: {file_path.name}")

    # ── Read and filter lines ─────────────────────────────────────────
    lines = filter_news_lines(file_path)
    if not lines:
        print_error("No valid news lines found in the file.")
        raise typer.Exit(code=1)

    console.print(f"[cyan]Found {len(lines)} news lines to classify[/cyan]")
    console.print(f"[dim]Topics: {len(STANDARD_TOPICS)} standard categories · Batch size: {batch_size}[/dim]")

    # ── Output path ───────────────────────────────────────────────────
    if output is None:
        output = file_path.parent / f"{file_path.stem}_parsed.xlsx"

    if output.exists():
        console.print(f"[yellow]⚡ Appending to existing file:[/yellow] {output}")
    else:
        console.print(f"[green]Creating new file:[/green] {output}")

    # ── LLM client (reuses existing LM Studio config) ─────────────────
    client = OpenAI(
        base_url=config.lm_studio_base_url,
        api_key=config.lm_studio_api_key,
    )

    # ── Process in batches ────────────────────────────────────────────
    all_records: list[dict] = []
    batches = [lines[i : i + batch_size] for i in range(0, len(lines), batch_size)]

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        MofNCompleteColumn(),
        console=console,
    ) as progress:
        task = progress.add_task("Classifying news…", total=len(batches))

        for batch_idx, batch in enumerate(batches):
            user_prompt = build_user_prompt(batch)

            try:
                response = client.chat.completions.create(
                    model=config.model_name,
                    messages=[
                        {"role": "system", "content": system_prompt},
                        {"role": "user", "content": user_prompt},
                    ],
                    temperature=0.1,
                )

                raw_output = response.choices[0].message.content or ""
                records = extract_json_array(raw_output)

                if len(records) != len(batch):
                    console.print(
                        f"[yellow]⚠ Batch {batch_idx + 1}: Expected {len(batch)} records, "
                        f"got {len(records)}. Padding/trimming.[/yellow]"
                    )
                    # Pad missing entries with raw text as fallback
                    while len(records) < len(batch):
                        i = len(records)
                        records.append({
                            "month": "Not Specified",
                            "year": "Not Specified",
                            "topic": "Miscellaneous",
                            "tags": "",
                            "news": batch[i].lstrip("- ").strip() if i < len(batch) else "PARSE ERROR",
                        })
                    records = records[: len(batch)]

                # Validate topics against standard list
                for rec in records:
                    if rec.get("topic") not in STANDARD_TOPICS:
                        rec["topic"] = "Miscellaneous"

                all_records.extend(records)

            except Exception as e:
                console.print(f"[red]✖ Batch {batch_idx + 1} failed: {e}[/red]")
                for line in batch:
                    all_records.append({
                        "month": "Not Specified",
                        "year": "Not Specified",
                        "topic": "Miscellaneous",
                        "tags": "",
                        "news": line.lstrip("- ").strip(),
                    })

            progress.advance(task)

    # ── Write / append Excel ──────────────────────────────────────────
    console.print(f"\n[cyan]Writing {len(all_records)} records to Excel…[/cyan]")
    write_excel(all_records, output, source_filename=file_path.name)

    print_success(f"Done! {len(all_records)} news items → {output}")
    console.print("[dim]Open in Excel → use column header dropdowns to filter by Month, Year, Topic, Tags[/dim]")
