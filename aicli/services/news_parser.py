"""
News Parser Service — Current Affairs → Filterable Excel.

Reads a text file where each line is one news item, classifies each via LLM
into (Month, Year, Topic, Tags, News), and writes a styled, filterable Excel file.
Supports APPEND mode so multiple source files accumulate into one master sheet.
"""

import json
import re
from pathlib import Path


# ──────────────────────────────────────────────────────────────────────
# Standardized Topic List (competitive‑exam categories)
# ──────────────────────────────────────────────────────────────────────

STANDARD_TOPICS: list[str] = [
    "Polity & Governance",
    "International Relations",
    "Defence & Security",
    "Economy & Finance",
    "Science & Technology",
    "Environment & Ecology",
    "Art, Culture & Heritage",
    "Awards & Honours",
    "Sports",
    "Appointments & Personnel",
    "Books & Authors",
    "Important Days & Dates",
    "Government Schemes & Programs",
    "Summits & Conferences",
    "Reports & Indices",
    "Obituaries",
    "Infrastructure & Energy",
    "Space & Exploration",
    "Health & Medicine",
    "Education",
    "Law & Judiciary",
    "Miscellaneous",
]


# ──────────────────────────────────────────────────────────────────────
# LLM Prompts
# ──────────────────────────────────────────────────────────────────────

def build_system_prompt() -> str:
    """Build the system prompt with the topic list baked in."""
    topics_block = "\n".join(f"  - {t}" for t in STANDARD_TOPICS)

    return f"""You are a precise Current Affairs classification engine for competitive exam preparation (UPSC/SSC/Banking).

You will receive one or more news items, each on its own numbered line. For EACH line, extract structured data and return it as a JSON array.

## OUTPUT FORMAT
Return a JSON array. Each element corresponds to one input line IN ORDER and must have exactly these 2 keys:
[
  {{
    "topic": "<Exactly one topic from the STANDARD LIST below>",
    "tags": "<Comma-separated micro-level keywords for granular filtering>"
  }}
]

## STANDARD TOPICS — USE EXACTLY ONE PER LINE
{topics_block}

## TOPIC CLASSIFICATION GUIDELINES
- Military exercises, missiles, warships, defense exports, military operations → "Defence & Security"
- Bilateral/multilateral relations, foreign visits, treaties, diplomatic agreements → "International Relations"
- UNESCO heritage, festivals, GI tags, traditional art forms, cultural events → "Art, Culture & Heritage"
- Person died / passed away / demise → "Obituaries"
- Person appointed / elected / nominated to a position → "Appointments & Personnel"
- Book launched / authored / released → "Books & Authors"
- Named day observances (World AIDS Day, Human Rights Day, etc.) → "Important Days & Dates"
- Government bills, acts, schemes, welfare missions, policy launches → "Government Schemes & Programs"
- Repo rate, GDP, UPI, financial systems, stock markets, budgets → "Economy & Finance"
- Satellites, rockets, ISRO/NASA space missions, astronauts → "Space & Exploration"
- Rankings, global indices, survey reports → "Reports & Indices"
- Sports tournaments, sports results, player records, auction → "Sports"
- Awards/prizes NOT sport-tournament results → "Awards & Honours"
- Dams, power plants, highways, airports, ports, energy projects → "Infrastructure & Energy"
- Biodiversity, climate, wildlife, conservation, pollution, Ramsar sites → "Environment & Ecology"
- Constitutional matters, governance reforms, renaming of institutions → "Polity & Governance"
- Disease control, WHO health declarations, medical breakthroughs → "Health & Medicine"
- Universities, education boards, academic events → "Education"
- Court judgments, impeachment, legal proceedings → "Law & Judiciary"
- COP/biodiversity/health summits, bilateral summits, international conferences → "Summits & Conferences"
- If genuinely ambiguous, use "Miscellaneous"

## TAGS RULES
- Tags are flexible, micro-level keywords that are MORE GRANULAR than the broad Topic.
- They help with fine-grained filtering beyond the standardized topic.
- Provide 2-5 comma-separated tags per news item.
- Tags should include: key entity names, geographic locations, organizations, specific sub-domains.
- Examples:
  - Topic "Defence & Security", Tags: "DRDO, Missile, K4, Nuclear, INS Arighat"
  - Topic "Sports", Tags: "Cricket, IPL, Auction, Cameron Green"
  - Topic "Environment & Ecology", Tags: "Ramsar, Wetland, Rajasthan, Siliserh Lake"
  - Topic "Art, Culture & Heritage", Tags: "UNESCO, Diwali, Intangible Heritage"
  - Topic "International Relations", Tags: "India, Russia, Summit, Putin, Delhi"
- Tags should be proper nouns or specific terms — NOT generic adjectives.

## STRICT OUTPUT RULES
- Output ONLY the raw JSON array. No markdown code fences, no explanation, no commentary.
- Every element must have all 2 keys.
- The array must have EXACTLY as many elements as input lines, in the same order."""


def build_user_prompt(lines: list[str]) -> str:
    """Build the user prompt with numbered lines."""
    numbered = "\n".join(f"{i + 1}. {line}" for i, line in enumerate(lines))
    return (
        f"Classify each of the following {len(lines)} news line(s). "
        f"Return a JSON array with exactly {len(lines)} element(s), one per line, in the same order.\n\n"
        f"{numbered}"
    )


# ──────────────────────────────────────────────────────────────────────
# Parsing helpers
# ──────────────────────────────────────────────────────────────────────

def extract_json_array(raw: str) -> list[dict]:
    """Robustly extract a JSON array from potentially noisy LLM output."""
    text = raw.strip()

    # Strip markdown code fences
    text = re.sub(r"^```(?:json)?\s*", "", text)
    text = re.sub(r"\s*```$", "", text)
    text = text.strip()

    # Attempt 1: direct parse
    try:
        parsed = json.loads(text)
        if isinstance(parsed, list):
            return parsed
    except json.JSONDecodeError:
        pass

    # Attempt 2: find the largest [...] block
    match = re.search(r"\[.*\]", text, re.DOTALL)
    if match:
        try:
            parsed = json.loads(match.group())
            if isinstance(parsed, list):
                return parsed
        except json.JSONDecodeError:
            pass

    return []


def filter_news_lines(filepath: Path) -> list[str]:
    """
    Read the file and return only lines that look like news items.
    Skips: empty lines and horizontal rules (---).
    """
    raw = filepath.read_text(encoding="utf-8")
    result: list[str] = []

    for line in raw.splitlines():
        stripped = line.strip()
        if not stripped:
            continue
        if stripped == "---":
            continue
        result.append(stripped)

    return result


def merge_duplicate_news(news_items: list[str], client, model_name: str) -> str:
    """
    Given a list of semantically similar news strings, use the LLM to merge them into a single, comprehensive string.
    Ensures zero loss of factual detail.
    """
    from rich.console import Console
    console = Console()
    
    system_prompt = (
        "You are an expert editor handling current affairs data.\n"
        "You will be given multiple news items that describe the exact same event.\n"
        "Your task is to merge them into a single cohesive entry.\n"
        "CRITICAL RULES:\n"
        "1. DO NOT simply summarize; you MUST merge EVERYTHING. Synthesize the inputs into a single narrative: remove redundant sentences or phrasing while ensuring EVERY UNIQUE fact, name, date, location, and statistic is retained.\n"
        "2. Retain EVERY SINGLE detail from ALL variations.\n"
        "3. Ensure the final merged output is well-formatted, professional, and is a coherent paragraph or set of bullets.\n"
        "4. DO NOT add any outside knowledge. Only use the provided facts.\n"
        "5. Output ONLY the perfectly merged text block. Do NOT wrap it in JSON. Do NOT include markdown quotes."
    )
    
    user_prompt = "Merge these overlapping news records into one:\n\n"
    for i, item in enumerate(news_items, 1):
        user_prompt += f"--- RECORD {i} ---\n{item}\n\n"
        
    try:
        response = client.chat.completions.create(
            model=model_name,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt},
            ],
            temperature=0.0,
        )
        content = response.choices[0].message.content.strip()
        return content
    except Exception as e:
        console.print(f"[red]⚠ AI Merge Error on cluster of size {len(news_items)}: {str(e)}[/red]")
        # Fallback to concatenation if AI merge fails
        return f"[MERGE ERROR - FALLBACK CONCATENATION]:\n\n" + "\n\n".join(news_items)


# ──────────────────────────────────────────────────────────────────────
# Excel writer — supports CREATE and APPEND
# ──────────────────────────────────────────────────────────────────────

HEADERS = ["S.No", "Month", "Year", "Topic", "Tags", "News", "Source", "Order"]
COL_WIDTHS = {"A": 6, "B": 14, "C": 8, "D": 30, "E": 36, "F": 100, "G": 20, "H": 10}


def _style_header_row(ws) -> None:
    """Apply professional styling to the header row."""
    from openpyxl.styles import Font, PatternFill, Alignment, Border, Side

    header_font = Font(name="Calibri", bold=True, color="FFFFFF", size=11)
    header_fill = PatternFill(start_color="1F3864", end_color="1F3864", fill_type="solid")
    header_align = Alignment(horizontal="center", vertical="center", wrap_text=True)
    thin_border = Border(
        left=Side(style="thin", color="8DB4E2"),
        right=Side(style="thin", color="8DB4E2"),
        top=Side(style="thin", color="8DB4E2"),
        bottom=Side(style="medium", color="1F3864"),
    )

    for col, header in enumerate(HEADERS, 1):
        cell = ws.cell(row=1, column=col, value=header)
        cell.font = header_font
        cell.fill = header_fill
        cell.alignment = header_align
        cell.border = thin_border


def _style_data_row(ws, row_num: int, values: list, row_index: int) -> None:
    """Apply styling to a single data row."""
    from openpyxl.styles import Font, PatternFill, Alignment, Border, Side

    data_font = Font(name="Calibri", size=10)
    even_fill = PatternFill(start_color="D6E4F0", end_color="D6E4F0", fill_type="solid")
    odd_fill = PatternFill(start_color="FFFFFF", end_color="FFFFFF", fill_type="solid")
    thin_border = Border(
        left=Side(style="thin", color="B4C6E7"),
        right=Side(style="thin", color="B4C6E7"),
        top=Side(style="thin", color="B4C6E7"),
        bottom=Side(style="thin", color="B4C6E7"),
    )

    fill = even_fill if row_index % 2 == 0 else odd_fill

    for col, value in enumerate(values, 1):
        cell = ws.cell(row=row_num, column=col, value=value)
        cell.font = data_font
        cell.fill = fill
        cell.border = thin_border
        if col in (1, 2, 3, 8):  # S.No, Month, Year, Order — centered
            cell.alignment = Alignment(horizontal="center", vertical="top")
        elif col == 6:  # News — wrap text
            cell.alignment = Alignment(vertical="top", wrap_text=True)
        else:
            cell.alignment = Alignment(vertical="top", wrap_text=True)


def _apply_finishing(ws, total_rows: int) -> None:
    """Apply auto-filter, freeze panes, and column widths."""
    ws.auto_filter.ref = f"A1:H{total_rows}"
    ws.freeze_panes = "A2"
    for col_letter, width in zip("ABCDEFGH", [6, 14, 8, 30, 36, 100, 20, 10]):
        ws.column_dimensions[col_letter].width = width


def write_excel(records: list[dict], output_path: Path, source_filename: str) -> None:
    """
    Write or APPEND parsed news records to a filterable Excel file.

    - If `output_path` does not exist → creates a new workbook.
    - If `output_path` already exists → loads it, appends new rows, re-numbers S.No.
    """
    from openpyxl import Workbook, load_workbook

    if output_path.exists():
        wb = load_workbook(output_path)
        ws = wb.active
        existing_rows = ws.max_row - 1  # subtract header
    else:
        wb = Workbook()
        ws = wb.active
        ws.title = "Current Affairs"
        _style_header_row(ws)
        existing_rows = 0

    # Append new data rows
    for idx, record in enumerate(records):
        global_idx = existing_rows + idx  # 0-based for zebra calc
        serial = existing_rows + idx + 1  # 1-base serial number
        row_num = serial + 1  # +1 for header

        values = [
            serial,
            record.get("month", "Not Specified"),
            record.get("year", "Not Specified"),
            record.get("topic", "Miscellaneous"),
            record.get("tags", ""),
            record.get("news", ""),
            record.get("source_key", ""),
            record.get("order_key", ""),
        ]
        _style_data_row(ws, row_num, values, global_idx)

    # Re-apply finishing touches (filter range expands with new rows)
    total_rows = existing_rows + len(records) + 1
    _apply_finishing(ws, total_rows)

    wb.save(output_path)
