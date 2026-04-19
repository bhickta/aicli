"""
News Reasoning Service
Handles LLM prompt generation, structured extraction, and AI summarization for news items.
"""
import json
import time
from pathlib import Path
from typing import List
from pydantic import BaseModel, Field
from langchain_core.prompts import PromptTemplate
from aicli.prompts.news_prompts import CLASSIFICATION_PROMPT, MERGE_PROMPT

class NewsItemSchema(BaseModel):
    topic: str = Field(description="Exactly one topic from the STANDARD TOPICS list.")
    tags: str = Field(description="Comma-separated micro-level keywords for granular filtering.")

class NewsClassificationResult(BaseModel):
    items: List[NewsItemSchema] = Field(description="List of classified news items exactly matching the input lines.")

class NewsReasoningService:
    @staticmethod
    def get_classification_messages(lines: list[str]) -> list:
        numbered = "\n".join(f"{i + 1}. {line}" for i, line in enumerate(lines))
        return CLASSIFICATION_PROMPT.format_messages(line_count=len(lines), numbered_lines=numbered)

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
    def merge_duplicate_news(news_items: list[str], provider) -> str:
        from rich.console import Console
        console = Console()

        raw_concatenation = "\n\n---\n".join(news_items)
        messages = MERGE_PROMPT.format_messages(raw_concatenation=raw_concatenation)
        system_str = messages[0].content
        user_str = messages[1].content

        try:
            return provider.complete_text(
                prompt=user_str,
                system_prompt=system_str,
                temperature=0.0,
                max_tokens=2048,
                allow_reasoning=True,
            )
        except Exception as e:
            console.print(f"[red]⚠ AI Merge Error on cluster of size {len(news_items)}: {str(e)}[/red]")
            return f"[MERGE ERROR - FALLBACK CONCATENATION]:\n\n" + raw_concatenation
