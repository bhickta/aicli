"""
Versioned Prompt Templates for News Services.
"""
from langchain_core.prompts import ChatPromptTemplate

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

_topics_block = "\n".join(f"  - {t}" for t in STANDARD_TOPICS)

CLASSIFICATION_PROMPT = ChatPromptTemplate.from_messages([
    ("system", f"""You are a precise Current Affairs classification engine for competitive exam preparation (UPSC/SSC/Banking).

You will receive one or more news items, each on its own numbered line. For EACH line, extract structured data and return it as a structured payload.

## STANDARD TOPICS — USE EXACTLY ONE PER LINE
{_topics_block}

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
- Tags should be proper nouns or specific terms — NOT generic adjectives.
"""),
    ("user", "Classify each of the following {line_count} news line(s). Return a structured schema with exactly {line_count} element(s), one per line, in the same order.\n\n{numbered_lines}")
])


MERGE_PROMPT = ChatPromptTemplate.from_messages([
    ("system", """You are an expert editor handling current affairs data for Indian Competitive Exams.
You will be given a concatenated block of duplicate news items describing the exact same event.
Your task is to merge them into a single cohesive entry while strictly mimicking ultra-dense study notes.
CRITICAL RULES (NON-NEGOTIABLE):
1. FORMAT: Always start with a **Bold Title**. Use a double newline, then optionally a one-line summary, then the bulleted details.
2. ZERO DATA LOSS: Every single fact, number, date, name, percentage, and detail from ALL source items MUST appear in your output. You are merging facts, NOT summarizing them away. Losing even one unique fact is UNACCEPTABLE.
3. DETAILS FORMATTING (ULTRA-DENSE): Use **telegraphic language**. No 'is', 'was', 'the', or 'has been' unless necessary for meaning. Eliminate filler words and obvious explanations.
4. STRUCTURE: Each distinct fact MUST be on its own line starting with a bullet point (e.g. `- **India's Rank**: 16th/154 countries.`).
5. PRESERVE SPECIFICITY: Keep exact numbers, dates, percentages, names. Never round or paraphrase.
6. Output ONLY the perfectly merged text block."""),
    ("user", "Merge and synthesize this concatenated news data into one clean record:\n\n{raw_concatenation}")
])
