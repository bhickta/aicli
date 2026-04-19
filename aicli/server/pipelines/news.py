"""
News Pipeline CLI Command Orchestrator.
Delegates heavy lifting to NewsReasoningService, NewsClusteringService, and NewsExcelRepository.
"""

import json
import threading
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed

import typer
from openai import OpenAI
from rich.progress import (
    Progress,
    SpinnerColumn,
    TextColumn,
    BarColumn,
    MofNCompleteColumn,
)

from aicli.cli.tui import print_header, print_success, print_error, console
from aicli.config import config
from aicli.server.services.news_reasoning_service import (
    NewsReasoningService,
    STANDARD_TOPICS,
)
from aicli.server.services.news_clustering_service import NewsClusteringService
from aicli.server.repositories.news_excel_repository import NewsExcelRepository


def _classify_batch(
    batch_idx: int, batch: list[str], system_prompt: str, client, model_name: str
) -> tuple[int, list[dict], Exception | None]:
    try:
        user_prompt = NewsReasoningService.build_user_prompt(batch)
        response = client.chat.completions.create(
            model=model_name,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.1,
        )
        raw_output = response.choices[0].message.content or ""
        records = NewsReasoningService.extract_json_array(raw_output)
        return batch_idx, records, None
    except Exception as e:
        return batch_idx, [], e


def parse_news(
    file_path: Path,
    output: Path = None,
    batch_size: int = 10,
    month: str = "Not Specified",
    year: str = "Not Specified",
    workers: int = 4,
    show_prompt: bool = False,
):
    system_prompt = NewsReasoningService.build_system_prompt()
    if show_prompt:
        console.print(system_prompt)
        raise typer.Exit()

    print_header(f"Parsing Current Affairs: {file_path.name}")
    lines = NewsReasoningService.filter_news_lines(file_path)
    if not lines:
        print_error("No valid news lines found in the file.")
        raise typer.Exit(code=1)

    console.print(f"[cyan]Found {len(lines)} news lines to classify[/cyan]")
    output = output or file_path.parent / f"{file_path.stem}_parsed.xlsx"
    client = OpenAI(
        base_url=f"{config.ollama_base_url}/v1", api_key=config.ollama_api_key
    )

    batches = [lines[i : i + batch_size] for i in range(0, len(lines), batch_size)]
    batch_results = {}

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        MofNCompleteColumn(),
        console=console,
    ) as progress:
        task = progress.add_task(
            f"Classifying news ({len(batches)} batches)…", total=len(batches)
        )
        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = {
                executor.submit(
                    _classify_batch,
                    idx,
                    batch,
                    system_prompt,
                    client,
                    config.model_name,
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
                    records = [{"topic": "Miscellaneous", "tags": ""} for _ in batch]
                elif len(records) != len(batch):
                    progress.console.print(
                        f"[yellow]⚠ Batch {batch_idx + 1}: Expected {len(batch)} records, got {len(records)}.[/yellow]"
                    )
                    while len(records) < len(batch):
                        records.append({"topic": "Miscellaneous", "tags": ""})
                    records = records[: len(batch)]
                else:
                    progress.console.print(
                        f"[green]✔ Batch {batch_idx + 1}: {len(records)} items classified[/green]"
                    )

                for i, rec in enumerate(records):
                    if rec.get("topic") not in STANDARD_TOPICS:
                        rec["topic"] = "Miscellaneous"
                    rec["month"] = month
                    rec["year"] = year
                    rec["news"] = batch[i].lstrip("- ").strip()

                batch_results[batch_idx] = records
                progress.advance(task)

    all_records = []
    for idx in range(len(batches)):
        all_records.extend(batch_results.get(idx, []))

    console.print(f"\n[cyan]Writing {len(all_records)} records to Excel…[/cyan]")
    NewsExcelRepository.write_excel(all_records, output)
    print_success(f"Done! {len(all_records)} news items → {output}")


def from_json(file_path: Path, output: Path = None):
    print_header(f"Parsing JSON: {file_path.name}")
    output = output or file_path.parent / f"{file_path.stem}.xlsx"

    try:
        data = json.loads(file_path.read_text(encoding="utf-8"))
    except Exception as e:
        print_error("Failed to read JSON.", e)
        raise typer.Exit(code=1)

    records = _json_to_records(data)
    NewsExcelRepository.write_excel(records, output)
    print_success(f"Done! {len(records)} items written to {output}")


def dedupe(
    file_path: Path, output: Path = None, threshold: float = 0.80, workers: int = 10
):
    print_header(f"AI De-duplication: {file_path.name}")
    output = output or file_path.parent / f"{file_path.stem}_deduped.xlsx"

    try:
        records = NewsExcelRepository.read_excel_records(file_path)
    except Exception as e:
        print_error("Failed to read Excel.", e)
        raise typer.Exit(code=1)

    if not records:
        print_error("No records found.")
        raise typer.Exit(code=1)

    console.print("[cyan]Running News Clustering Service...[/cyan]")
    cluster_service = NewsClusteringService(threshold=threshold)
    clusters, num_duplicates = cluster_service.cluster_records(records)

    if num_duplicates == 0:
        console.print("[green]✔ No duplicates found! Excel is perfectly clean.[/green]")
        raise typer.Exit()

    console.print(
        f"[yellow]⚠ Found {len(clusters)} unique events. {num_duplicates} duplicate records will be merged.[/yellow]"
    )

    client = OpenAI(
        base_url=f"{config.ollama_base_url}/v1", api_key=config.ollama_api_key
    )
    unique_records = _run_ai_merging(clusters, records, client, workers, output)

    if output.exists():
        output.unlink()
    NewsExcelRepository.write_excel(unique_records, output)
    print_success(f"Deduplicated file saved → {output}")


def process_news(
    json_path: Path,
    output: Path = None,
    workers: int = 4,
    threshold: float = 0.8,
    force_merge: bool = False,
    no_cache: bool = False,
):
    print_header("God-Mode Processing")
    console.print(f"[cyan]Parsing input JSON: {json_path.name}...[/cyan]")
    output = output or json_path.parent / f"{json_path.stem}_master.xlsx"

    data = json.loads(json_path.read_text(encoding="utf-8"))
    records = _json_to_records(data)
    console.print(f"[green]✔ Extracted {len(records)} new records from JSON.[/green]")

    if output.exists():
        existing = NewsExcelRepository.read_excel_records(output)
        records.extend(existing)
        console.print(
            f"[green]✔ Loaded {len(existing)} existing records. Total pool: {len(records)} items.[/green]"
        )

    cluster_service = NewsClusteringService(threshold=threshold)
    clusters, num_duplicates = cluster_service.cluster_records(records)

    if num_duplicates == 0:
        console.print("[green]✔ No duplicates found! Pool is perfectly clean.[/green]")
        if output.exists():
            output.unlink()
        NewsExcelRepository.write_excel(records, output)
        raise typer.Exit()

    client = OpenAI(
        base_url=f"{config.ollama_base_url}/v1", api_key=config.ollama_api_key
    )
    unique_records = _run_ai_merging(
        clusters, records, client, workers, output, force_merge=force_merge
    )

    if output.exists():
        output.unlink()
    NewsExcelRepository.write_excel(unique_records, output)
    print_success(f"File updated and saved → {output}")


# ───── Private Helpers ────────────────────────────────────────────────────────
def _json_to_records(data: list) -> list[dict]:
    records = []
    for item in data:
        title = item.get("title", "").strip()
        key_answer = item.get("key_answer", "").strip()
        details = item.get("details", "").strip()

        news_str = "\n".join(
            [p for p in [f"**{title}**" if title else "", key_answer, details] if p]
        )
        source = str(item.get("Source") or item.get("source", "")).strip()
        order = str(item.get("Order") or item.get("order", "")).strip()
        topic = item.get("category", "")
        if topic not in STANDARD_TOPICS:
            topic = topic if topic else "Miscellaneous"

        sources_dict = {source: order} if source else {}
        item_json = json.dumps(
            {
                "source": source,
                "order": order,
                "news": news_str,
                "title": title,
                "topic": topic,
                "tags": str(item.get("tags", "")),
            }
        )

        records.append(
            {
                "date": f"{item.get('month', 'Not Specified')} - {item.get('year', 'Not Specified')}",
                "topic": topic,
                "tags": str(item.get("tags", "")),
                "news": news_str,
                "sources": sources_dict,
                "concat_json": item_json,
            }
        )
    return records


def _run_ai_merging(
    clusters: list[list[int]],
    records: list[dict],
    client,
    workers: int,
    output: Path,
    force_merge: bool = False,
) -> list[dict]:
    unique_records = []
    merge_clusters = [c for c in clusters if len(c) > 1]
    for c in [c for c in clusters if len(c) == 1]:
        unique_records.append(records[c[0]])

    merge_jobs = []
    for cluster in merge_clusters:
        items_to_merge = [records[idx] for idx in cluster]
        t_tags = []
        date_from_cluster = ""
        topic = "Miscellaneous"
        pairs = []
        for item in items_to_merge:
            t_tags.extend([t.strip() for t in item["tags"].split(",") if t.strip()])
            if item.get("topic") and item["topic"] != "Miscellaneous":
                topic = item["topic"]
            if item.get("date") and item["date"] != "Not Specified":
                date_from_cluster = item["date"]
            # Simple source_key extraction
            sp = [s.strip() for s in item.get("source_key", "").split("|")]
            for s in sp:
                if s:
                    pairs.append((s, ""))

        unique_tags = list(dict.fromkeys(t_tags))
        merged_source_key = " | ".join(p[0] for p in pairs if p[0])

        merge_jobs.append(
            {
                "date": date_from_cluster or "Not Specified",
                "topic": topic,
                "tags": ", ".join(unique_tags),
                "source_key": merged_source_key,
                "_raw_order": "",
                "_news_strings": [i["news"] for i in items_to_merge],
            }
        )

    save_lock = threading.Lock()

    def _save_snapshot(recs):
        with save_lock:
            if output.exists():
                output.unlink()
            NewsExcelRepository.write_excel(list(recs), output)

    def _do_merge(job):
        merged = NewsReasoningService.merge_duplicate_news(
            job.pop("_news_strings"), client, config.model_name
        )
        job["news"] = merged
        return job

    if output.exists():
        output.unlink()
    _save_snapshot(unique_records)

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        MofNCompleteColumn(),
        console=console,
    ) as progress:
        task = progress.add_task(
            f"Merging {len(merge_jobs)} duplicates...", total=len(merge_jobs)
        )
        with ThreadPoolExecutor(max_workers=workers) as executor:
            futures = {
                executor.submit(_do_merge, job): i for i, job in enumerate(merge_jobs)
            }
            for future in as_completed(futures):
                unique_records.append(future.result())
                progress.advance(task)
                _save_snapshot(unique_records)

    unique_records.sort(
        key=lambda x: (
            x.get("source_key", "").split("|")[0].split("-")[0].strip().lower(),
            999999,
        )
    )
    return unique_records
