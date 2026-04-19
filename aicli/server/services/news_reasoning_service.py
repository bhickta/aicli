"""
News Reasoning Service
Handles LLM prompt generation, structured extraction, and AI summarization for news items.
"""
import json
import re
import time
from pathlib import Path

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


class NewsReasoningService:
    @staticmethod
    def build_system_prompt() -> str:
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

    @staticmethod
    def build_user_prompt(lines: list[str]) -> str:
        numbered = "\n".join(f"{i + 1}. {line}" for i, line in enumerate(lines))
        return (
            f"Classify each of the following {len(lines)} news line(s). "
            f"Return a JSON array with exactly {len(lines)} element(s), one per line, in the same order.\n\n"
            f"{numbered}"
        )

    @staticmethod
    def extract_json_array(raw: str) -> list[dict]:
        text = raw.strip()
        text = re.sub(r"^```(?:json)?\s*", "", text)
        text = re.sub(r"\s*```$", "", text)
        text = text.strip()

        try:
            parsed = json.loads(text)
            if isinstance(parsed, list):
                return parsed
        except json.JSONDecodeError:
            pass

        match = re.search(r"\[.*\]", text, re.DOTALL)
        if match:
            try:
                parsed = json.loads(match.group())
                if isinstance(parsed, list):
                    return parsed
            except json.JSONDecodeError:
                pass
        return []

    @staticmethod
    def filter_news_lines(filepath: Path) -> list[str]:
        raw = filepath.read_text(encoding="utf-8")
        result: list[str] = []
        for line in raw.splitlines():
            stripped = line.strip()
            if not stripped or stripped == "---":
                continue
            result.append(stripped)
        return result

    @staticmethod
    def merge_duplicate_news(news_items: list[str], client, model_name: str) -> str:
        from rich.console import Console
        console = Console()

        raw_concatenation = "\n\n---\n".join(news_items)

        system_prompt = (
            "You are an expert editor handling current affairs data for Indian Competitive Exams.\n"
            "You will be given a concatenated block of duplicate news items describing the exact same event.\n"
            "Your task is to merge them into a single cohesive entry while strictly mimicking ultra-dense study notes.\n"
            "CRITICAL RULES (NON-NEGOTIABLE):\n"
            "1. FORMAT: Always start with a **Bold Title**. Use a double newline, then optionally a one-line summary, then the bulleted details.\n"
            "2. ZERO DATA LOSS: Every single fact, number, date, name, percentage, and detail from ALL source items MUST appear in your output. You are merging facts, NOT summarizing them away. Losing even one unique fact is UNACCEPTABLE.\n"
            "3. DETAILS FORMATTING (ULTRA-DENSE): Use **telegraphic language**. No 'is', 'was', 'the', or 'has been' unless necessary for meaning. Eliminate filler words and obvious explanations.\n"
            "4. STRUCTURE: Each distinct fact MUST be on its own line starting with a bullet point (e.g. `- **India's Rank**: 16th/154 countries.`).\n"
            "5. PRESERVE SPECIFICITY: Keep exact numbers, dates, percentages, names. Never round or paraphrase.\n"
            "6. Output ONLY the perfectly merged text block. Do NOT include markdown code fences (like ```), JSON, or conversational replies."
        )

        user_prompt = f"Merge and synthesize this concatenated news data into one clean record:\n\n{raw_concatenation}"

        max_retries = 5
        base_delay = 2.0

        for attempt in range(max_retries):
            try:
                response = client.chat.completions.create(
                    model=model_name,
                    messages=[
                        {"role": "system", "content": system_prompt},
                        {"role": "user", "content": user_prompt},
                    ],
                    temperature=0.0,
                )
                return response.choices[0].message.content.strip()

            except Exception as e:
                if attempt == max_retries - 1:
                    console.print(f"[red]⚠ AI Merge Error on cluster of size {len(news_items)} after {max_retries} attempts: {str(e)}[/red]")
                    return f"[MERGE ERROR - FALLBACK CONCATENATION]:\n\n" + raw_concatenation

                err_msg = str(e).lower()
                if any(x in err_msg for x in ["model unloaded", "context size", "no models loaded", "failed to parse"]):
                    time.sleep(base_delay * (2 ** attempt))
                else:
                    time.sleep(base_delay)
