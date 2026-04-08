import typer
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, MofNCompleteColumn
from aicli.cli.tui import print_header, print_success, print_error, console

app = typer.Typer(help="Current affairs news parsing → filterable Excel.")


def _classify_batch(
    batch_idx: int,
    batch: list[str],
    system_prompt: str,
    client,
    model_name: str,
) -> tuple[int, list[dict], Exception | None]:
    """Worker function: sends one batch to the LLM and returns parsed records."""
    from aicli.services.news_parser import build_user_prompt, extract_json_array

    try:
        user_prompt = build_user_prompt(batch)
        response = client.chat.completions.create(
            model=model_name,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.1,
        )

        raw_output = response.choices[0].message.content or ""
        records = extract_json_array(raw_output)
        return batch_idx, records, None
    except Exception as e:
        return batch_idx, [], e


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
    month: str = typer.Option(
        "Not Specified",
        "--month", "-m",
        help="Global month to apply to all items (e.g. 'December').",
    ),
    year: str = typer.Option(
        "Not Specified",
        "--year", "-y",
        help="Global year to apply to all items (e.g. '2025').",
    ),
    workers: int = typer.Option(
        4,
        "--workers",
        "-W",
        help="Number of parallel LLM inference threads.",
        min=1,
        max=8,
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
    each into Topic and Tags. Month and Year are supplied via CLI options.
    The original line is preserved completely as the News block.

    Supports APPEND mode: run on multiple files with the same --output and all
    rows accumulate into a single master Excel sheet.
    """
    from openai import OpenAI
    from aicli.config import config
    from aicli.services.news_parser import (
        build_system_prompt,
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
    console.print(f"[dim]Topics: {len(STANDARD_TOPICS)} · Config: Month={month}, Year={year}[/dim]")

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

    # ── Split into batches ────────────────────────────────────────────
    batches = [lines[i : i + batch_size] for i in range(0, len(lines), batch_size)]

    # ── Parallel processing ───────────────────────────────────────────
    # Results indexed by batch_idx to maintain original line order
    batch_results: dict[int, list[dict]] = {}

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        MofNCompleteColumn(),
        console=console,
    ) as progress:
        task = progress.add_task(f"Classifying news ({len(batches)} batches)…", total=len(batches))

        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = {
                executor.submit(
                    _classify_batch, idx, batch, system_prompt, client, config.model_name
                ): (idx, batch)
                for idx, batch in enumerate(batches)
            }

            for future in as_completed(futures):
                idx, batch = futures[future]
                batch_idx, records, err = future.result()

                if err:
                    progress.console.print(
                        f"[red]✖ Batch {batch_idx + 1} failed: {err}[/red]"
                    )
                    # Fallback: use raw text for failed batches
                    records = [
                        {
                            "topic": "Miscellaneous",
                            "tags": "",
                        }
                        for _ in batch
                    ]

                elif len(records) != len(batch):
                    progress.console.print(
                        f"[yellow]⚠ Batch {batch_idx + 1}: Expected {len(batch)} records, "
                        f"got {len(records)}. Padding/trimming.[/yellow]"
                    )
                    while len(records) < len(batch):
                        records.append({
                            "topic": "Miscellaneous",
                            "tags": "",
                        })
                    records = records[: len(batch)]
                else:
                    progress.console.print(
                        f"[green]✔ Batch {batch_idx + 1}: {len(records)} items classified[/green]"
                    )

                # Validate topics and inject month/year/news
                for i, rec in enumerate(records):
                    if rec.get("topic") not in STANDARD_TOPICS:
                        rec["topic"] = "Miscellaneous"
                    
                    # Inject CLI-provided and original line data
                    rec["month"] = month
                    rec["year"] = year
                    rec["news"] = batch[i].lstrip("- ").strip()

                batch_results[batch_idx] = records
                progress.advance(task)

    # ── Reassemble in original order ──────────────────────────────────
    all_records: list[dict] = []
    for idx in range(len(batches)):
        all_records.extend(batch_results.get(idx, []))

    # ── Write / append Excel ──────────────────────────────────────────
    console.print(f"\n[cyan]Writing {len(all_records)} records to Excel…[/cyan]")
    write_excel(all_records, output, source_filename=file_path.name)

    print_success(f"Done! {len(all_records)} news items → {output}")
    console.print("[dim]Open in Excel → use column header dropdowns to filter by Month, Year, Topic, Tags[/dim]")


@app.command("from-json")
def from_json(
    file_path: Path = typer.Argument(
        ...,
        exists=True,
        file_okay=True,
        dir_okay=False,
        help="Path to the JSON file containing current affairs data.",
    ),
    output: Path = typer.Option(
        None,
        "--output", "-o",
        help="Output Excel file path. Defaults to <input_name>.xlsx. Appends if exists.",
    )
):
    """
    Convert a generated JSON file directly into the filterable Excel spreadsheet.
    Requires no AI — purely deterministic parsing.
    """
    import json
    from aicli.services.news_parser import write_excel, STANDARD_TOPICS

    print_header(f"Parsing JSON: {file_path.name}")
    
    if output is None:
        output = file_path.parent / f"{file_path.stem}.xlsx"

    try:
        data = json.loads(file_path.read_text(encoding="utf-8"))
    except Exception as e:
        print_error("Failed to read JSON file.", e)
        raise typer.Exit(code=1)

    if not isinstance(data, list):
        print_error("JSON file must contain an array of objects.")
        raise typer.Exit(code=1)

    records = []
    for item in data:
        # Construct News string
        title = item.get("title", "").strip()
        key_answer = item.get("key_answer", "").strip()
        details = item.get("details", "").strip()
        
        news_parts = []
        if title:
            news_parts.append(f"**{title}**")
        if key_answer:
            news_parts.append(key_answer)
        if details:
            news_parts.append(details)
            
        news_str = "\n".join(news_parts)
        
        # Read keys (support upper and lower case added by user)
        source = item.get("Source") or item.get("source", "")
        order = item.get("Order") or item.get("order", "")
        
        # Standardize topic if possible
        topic = item.get("category", "")
        if topic not in STANDARD_TOPICS:
            if topic: 
                pass # keep whatever is in JSON if present
            else:
                topic = "Miscellaneous"

        records.append({
            "month": item.get("month", "Not Specified"),
            "year": str(item.get("year", "Not Specified")),
            "topic": topic,
            "tags": str(item.get("tags", "")),
            "news": news_str,
            "source_key": str(source),
            "order_key": str(order)
        })

    if output.exists():
        console.print(f"[yellow]⚡ Appending {len(records)} records to existing file:[/yellow] {output}")
    else:
        console.print(f"[green]Creating new file with {len(records)} records:[/green] {output}")

    write_excel(records, output, source_filename="")
    print_success(f"Done! {len(records)} items written to {output}")


@app.command("dedupe")
def dedupe(
    file_path: Path = typer.Argument(
        ...,
        exists=True,
        file_okay=True,
        dir_okay=False,
        help="Path to the Excel file to deduplicate.",
    ),
    output: Path = typer.Option(
        None,
        "--output", "-o",
        help="Output Excel file path. Defaults to <input_name>_deduped.xlsx.",
    ),
    threshold: float = typer.Option(
        0.85,
        "--threshold", "-t",
        help="Cosine similarity threshold for embeddings (0.0 to 1.0). Default is 0.85.",
    ),
):
    """
    De-duplicate an existing Current Affairs Excel file using RAG AI.
    
    1. Computes local deep semantic embeddings for every row using sentence-transformers.
    2. Clusters similar rows together via Cosine Similarity.
    3. Calls your local LLM to perfectly merge the duplicate news entries without data loss.
    """
    import openpyxl
    from openai import OpenAI
    from sentence_transformers import SentenceTransformer, util
    
    from aicli.config import config
    from aicli.services.news_parser import write_excel, merge_duplicate_news
    
    print_header(f"AI De-duplication: {file_path.name}")
    
    if output is None:
        output = file_path.parent / f"{file_path.stem}_deduped.xlsx"
        
    try:
        wb = openpyxl.load_workbook(file_path)
        ws = wb.active
    except Exception as e:
        print_error("Failed to read Excel file.", e)
        raise typer.Exit(code=1)
        
    records = []
    for row in ws.iter_rows(min_row=2, values_only=True):
        if not any(row):
            continue
            
        records.append({
            "month": str(row[1]) if len(row)>1 and row[1] is not None else "Not Specified",
            "year": str(row[2]) if len(row)>2 and row[2] is not None else "Not Specified",
            "topic": str(row[3]) if len(row)>3 and row[3] is not None else "Miscellaneous",
            "tags": str(row[4]) if len(row)>4 and row[4] is not None else "",
            "news": str(row[5]) if len(row)>5 and row[5] is not None else "",
            "source_key": str(row[6]) if len(row)>6 and row[6] is not None else "",
            "order_key": str(row[7]) if len(row)>7 and row[7] is not None else "",
        })
        
    if not records:
        print_error("No records found in Excel.")
        raise typer.Exit(code=1)

    # ── 1. Embeddings ────────────────────────────────────────────────
    console.print(f"[cyan]Loading local embedding model ('all-MiniLM-L6-v2')...[/cyan]")
    model = SentenceTransformer('all-MiniLM-L6-v2')
    
    news_texts = [r["news"] for r in records]
    console.print(f"[cyan]Computing semantic embeddings for {len(records)} records...[/cyan]")
    embeddings = model.encode(news_texts, convert_to_tensor=True)
    
    # ── 2. Clustering ────────────────────────────────────────────────
    console.print(f"[cyan]Clustering duplicates (Threshold > {threshold})...[/cyan]")
    cos_scores = util.cos_sim(embeddings, embeddings)
    
    visited = set()
    clusters = []
    
    for i in range(len(records)):
        if i in visited: continue
        cluster = [i]
        visited.add(i)
        for j in range(i + 1, len(records)):
            if j not in visited and cos_scores[i][j].item() >= threshold:
                cluster.append(j)
                visited.add(j)
        clusters.append(cluster)
        
    num_duplicates = sum(len(c) - 1 for c in clusters)
    if num_duplicates == 0:
        console.print("[green]✔ No duplicates found! Excel is perfectly clean.[/green]")
        raise typer.Exit()
        
    console.print(f"[yellow]⚠ Found {len(clusters)} unique events. {num_duplicates} duplicate records will be merged.[/yellow]")

    # ── 3. AI Merging ────────────────────────────────────────────────
    client = OpenAI(
        base_url=config.lm_studio_base_url,
        api_key=config.lm_studio_api_key,
    )
    
    unique_records = []
    
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        MofNCompleteColumn(),
        console=console,
    ) as progress:
        task = progress.add_task("Merging duplicates via LLM...", total=len(clusters))
        
        for cluster in clusters:
            items_to_merge = [records[idx] for idx in cluster]
            
            # Base case: no duplicate
            if len(items_to_merge) == 1:
                unique_records.append(items_to_merge[0])
                progress.advance(task)
                continue
                
            # Merge case
            t_tags = []
            month = ""
            year = ""
            topic = "Miscellaneous"
            
            pairs = []
            seen = set()
            
            for item in items_to_merge:
                t_tags.extend([t.strip() for t in item["tags"].split(",") if t.strip()])
                
                if item["month"] and item["month"] != "Not Specified": month = item["month"]
                if item["year"] and item["year"] != "Not Specified": year = item["year"]
                if item["topic"] and item["topic"] != "Miscellaneous": topic = item["topic"]
                
                sp = [s.strip() for s in item.get("source_key", "").split("|")]
                op = [o.strip() for o in item.get("order_key", "").split("|")]
                max_l = max(len(sp), len(op))
                sp += [""] * (max_l - len(sp))
                op += [""] * (max_l - len(op))
                for s, o in zip(sp, op):
                    if (s, o) not in seen and (s or o):
                        seen.add((s, o))
                        pairs.append((s, o))

            unique_tags = list(dict.fromkeys(t_tags))
            merged_source = " | ".join(p[0] for p in pairs)
            merged_order = " | ".join(p[1] for p in pairs)
            
            news_strings = [i["news"] for i in items_to_merge]
            merged_news = merge_duplicate_news(news_strings, client, config.model_name)
            
            unique_records.append({
                "month": month or "Not Specified",
                "year": year or "Not Specified",
                "topic": topic,
                "tags": ", ".join(unique_tags),
                "news": merged_news,
                "source_key": merged_source,
                "order_key": merged_order
            })
            
            progress.advance(task)

    console.print(f"[green]✔ AI De-duplication complete. Merged {num_duplicates} duplicate records.[/green]")
    console.print(f"[cyan]Writing {len(unique_records)} pristine records to Excel...[/cyan]")
    
    # Must remove the output file if it exists, because write_excel appends if it exists
    if output.exists():
        output.unlink()
        
    write_excel(unique_records, output, source_filename="")
    print_success(f"Deduplicated file saved → {output}")
