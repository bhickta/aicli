from pathlib import Path
from typing import Dict, Any, List

from pydantic import BaseModel, Field
from langchain_core.prompts import PromptTemplate

from aicli.providers import get_provider


class VideoTaggerService:
    """High-level Orchestrator for video metadata generation."""

    VIDEO_EXTENSIONS = {".mp4", ".mkv", ".mov", ".avi", ".webm", ".m4v", ".ts", ".mts"}

    @staticmethod
    def ask_ollama(
        clips: List[Dict[str, Any]], path_hint: str, allow_reasoning: bool = False
    ) -> Dict[str, Any]:
        
        class TaggerSchema(BaseModel):
            title: str = Field(description="Descriptive title for the lecture (max 60 chars)")
            filename: str = Field(description="Clean, human-readable filename in Title Case with spaces (max 50 chars, NO hyphens, NO underscores)")
            lesson_number: int = Field(description="Actual chronological lesson, class, or sequence number. 999 if uncertain.")
            subject: str = Field(description="Academic subject (e.g. Physics, History, Math)")
            topics: List[str] = Field(description="List of 3 main topics")
            description: str = Field(description="2-3 sentence summary of the core concepts taught")
            teacher: str = Field(description="Name of the teacher or professor ('Unknown' if none)")
            coaching: str = Field(description="Coaching institute or channel name ('Unknown' if none)")
            language: str = Field(description="Spoken language (e.g. hin, eng)")

        transcript = "\n".join(
            f"[{int(c['start_sec'] // 60)}m{int(c['start_sec'] % 60)}s] {c['text']}"
            for c in clips
        )

        system = """You are a highly efficient JSON data-extraction engine.
You extract structured metadata from raw video transcripts.
CRUCIAL: Output ONLY valid data per the schema instructions."""

        user_prompt = "Folder: {path_hint}\n\nTranscript Snippet:\n{transcript}\n\nGenerate structured data."

        provider = get_provider()
        
        prompt_template = PromptTemplate.from_template(user_prompt)
        rendered_prompt = prompt_template.format(path_hint=path_hint, transcript=transcript[:4000])

        try:
            result = provider.structured_invoke(
                schema=TaggerSchema,
                prompt=rendered_prompt,
                system_prompt=system,
                allow_reasoning=allow_reasoning,
            )
            return result.model_dump()
        except Exception as e:
            raise ValueError(f"Failed to fetch or parse video tags: {e}")

    @staticmethod
    def global_course_sort(videos: List[Dict[str, Any]], allow_reasoning: bool = False) -> List[str]:
        """
        Pass filenames to Provider for intelligent chronological ordering.
        """
        
        class SortSchema(BaseModel):
            sorted_indices: List[int] = Field(description="Integer indices of the files strictly sorted in correct chronological course order")

        system = """You are an academic curriculum sequencer.
Given a numbered list of lecture video filenames, output their integer IDs sorted in correct chronological course order.
Use lesson numbers (L02, L23, LESSON_12), unit numbers (UNIT_1, unit-8), and part numbers to determine sequence."""

        filenames = [v["path"] for v in videos]
        numbered_list = []
        for i, v in enumerate(videos):
            ai = v.get("ai", {})
            title = ai.get("title", "Unknown")
            lesson = ai.get("lesson_number", "??")
            stem = Path(v["path"]).stem
            numbered_list.append(f"ID {i}: [Lesson {lesson}] {title} (File: {stem})")
        
        context_block = "\n".join(numbered_list)

        provider = get_provider()
        user_prompt = "COURSE CONTENT TO SEQUENCE:\n{context_block}\n\nTASK: Output the integer IDs in strict chronological order. Output struct:"
        
        prompt_template = PromptTemplate.from_template(user_prompt)
        rendered_prompt = prompt_template.format(context_block=context_block)

        try:
            result = provider.structured_invoke(
                schema=SortSchema,
                prompt=rendered_prompt,
                system_prompt=system,
                allow_reasoning=allow_reasoning,
            )
            
            sorted_indices = result.sorted_indices
            return [
                filenames[i]
                for i in sorted_indices
                if 0 <= i < len(filenames)
            ]
        except Exception as e:
            raise RuntimeError(f"LLM global sort failed: {e}") from e
