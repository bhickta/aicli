import os
import re
from pathlib import Path
from aicli.config import config as app_config
from aicli.services.image.prompts import (
    IMAGE_RENAME_SYSTEM_PROMPT,
    IMAGE_RENAME_USER_PROMPT,
    JUNK_FILTER_PROMPT,
    STRICT_JUNK_PROMPT,
    LAX_JUNK_PROMPT,
    MARKDOWN_CONVERSION_PROMPT
)
from aicli.core.interfaces import ImageVisionProvider


class ImageRenamerService:
    """
    High-level service that coordinates with an ImageVisionProvider to generate
    a new descriptive kebab-case name for an image, then applies the rename securely.
    """

    # Word count bounds for the generated filename
    MIN_WORDS = 3
    MAX_WORDS = 6

    def __init__(self, ai_provider: ImageVisionProvider):
        # Dependency Injection — adhering to SOLID principles
        self.provider = ai_provider

    # ------------------------------------------------------------------
    # Prompt construction
    # ------------------------------------------------------------------

    @staticmethod
    def _get_prompts(trash_junk: bool = False):
        min_w = app_config.image_rename_min_words
        max_w = app_config.image_rename_max_words
        
        sys_p = IMAGE_RENAME_SYSTEM_PROMPT.format(min_words=min_w, max_words=max_w)
        if trash_junk:
            sys_p += JUNK_FILTER_PROMPT
            
        user_p = IMAGE_RENAME_USER_PROMPT.format(min_words=min_w, max_words=max_w)
        
        return sys_p, user_p

    # ------------------------------------------------------------------
    # Core name generation
    # ------------------------------------------------------------------

    def generate_new_name(self, image_path: str, trash_junk: bool = False) -> str:
        """
        Uses the vision provider to analyze the image and suggest a new concise
        kebab-case filename (without extension).

        Returns:
            A sanitized kebab-case string, e.g. 'golden-retriever-muddy-paws'.

        Raises:
            ValueError: If a valid kebab-case name cannot be extracted from the model output.
        """
        sys_p, user_p = self._get_prompts(trash_junk)
        raw_response = self.provider.describe_image(
            image_path,
            user_p,
            system_prompt=sys_p,
        )

        cleaned = self._post_process(raw_response)

        if not cleaned:
            raise ValueError(
                f"Could not extract a valid kebab-case name from model response: {raw_response!r}"
            )

        return cleaned

    # ------------------------------------------------------------------
    # Post-processing pipeline
    # ------------------------------------------------------------------

    def _post_process(self, raw: str) -> str:
        """
        Multi-stage cleaning pipeline that turns chatty or malformed LLM output
        into a valid kebab-case filename segment.

        Stages:
            1. Strip surrounding whitespace and common wrapper characters.
            2. Collapse the output if the model returned multiple lines.
            3. Extract the best kebab-case token if prose crept in.
            4. Enforce character whitelist (a-z, 0-9, hyphen).
            5. Collapse/strip leading and trailing hyphens.
            6. Enforce word-count bounds.
        """
        # Check for TRASH early bypass
        raw_upper = raw.strip().strip("`\"'").upper()
        if raw_upper == "TRASH":
            return "TRASH"
            
        # Stage 1 — basic strip
        text = raw.strip().strip("`\"'")

        # Stage 2 — collapse multi-line output; keep only the first non-empty line
        lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
        text = lines[0] if lines else text

        # Stage 3 — prose detection & kebab extraction
        text = self._extract_kebab_from_prose(text)

        # Stage 4 — whitelist filter (lowercase alphanumeric + hyphen only)
        text = text.lower()
        text = re.sub(r"[^a-z0-9-]", "-", text)   # replace illegal chars with hyphen
        text = re.sub(r"-{2,}", "-", text)          # collapse consecutive hyphens
        text = text.strip("-")                      # remove leading/trailing hyphens

        # Stage 5 — word-count enforcement
        text = self._enforce_word_count(text)

        return text

    @staticmethod
    def _extract_kebab_from_prose(text: str) -> str:
        """
        If the model output looks like a sentence or contains a mix of prose and
        a kebab token, try to isolate the most useful kebab-case token.

        Strategy:
            - If the text already looks like a clean kebab slug, return as-is.
            - Otherwise tokenise on whitespace and prefer the longest hyphenated word.
            - Final fallback: smash the last N content words together.
        """
        # Already looks like a clean slug — no spaces, no sentence structure
        if " " not in text and "\n" not in text:
            return text

        words = text.split()

        # Remove common prose lead-ins like "Here is the name:" or "Filename:"
        filler_patterns = re.compile(
            r"^(here\s+(is|are)|the\s+filename|filename|name|output|result|answer)\b.*",
            re.IGNORECASE,
        )
        words = [w for w in words if not filler_patterns.match(w)]

        # Prefer the longest hyphenated token — most likely the actual slug
        hyphenated = [w for w in words if "-" in w]
        if hyphenated:
            return sorted(hyphenated, key=len, reverse=True)[0]

        # Fallback: join last meaningful words into a slug
        content_words = [
            w.lower() for w in words
            if w.lower() not in {"a", "an", "the", "of", "with", "in", "on", "is", "are"}
        ]
        min_w = app_config.image_rename_min_words
        slug_words = content_words[-min_w:]
        return "-".join(slug_words) if slug_words else text

    @staticmethod
    def _enforce_word_count(slug: str) -> str:
        """
        Ensures the slug falls within [min, max] hyphen-separated tokens.
        Truncates if too long; returns as-is if within bounds or too short.
        """
        parts = slug.split("-")
        max_w = app_config.image_rename_max_words
        if len(parts) > max_w:
            parts = parts[:max_w]
        return "-".join(parts)

    # ------------------------------------------------------------------
    # File system operation
    # ------------------------------------------------------------------

    def apply_rename(self, image_path: str, new_name_without_ext: str) -> str:
        """
        Renames the file on disk and returns the new absolute path.

        Args:
            image_path:           Absolute or relative path to the existing image file.
            new_name_without_ext: Sanitized kebab-case name without extension.

        Returns:
            The new absolute path as a string.

        Raises:
            FileNotFoundError: If the source file does not exist.
            FileExistsError:   If a file with the new name already exists in the same directory.
        """
        path_obj = Path(image_path).resolve()

        if not path_obj.exists():
            raise FileNotFoundError(f"Image not found: {image_path}")

        ext = path_obj.suffix
        new_filename = f"{new_name_without_ext}{ext}"
        new_path = path_obj.parent / new_filename

        # If the file already has this exact name, do nothing successfully
        if path_obj == new_path:
            return str(new_path)

        # Handle collisions if multiple images get the same name
        counter = 1
        while new_path.exists():
            new_filename = f"{new_name_without_ext}-{counter}{ext}"
            new_path = path_obj.parent / new_filename
            counter += 1

        os.rename(path_obj, new_path)
        return str(new_path)

    # ------------------------------------------------------------------
    # Convenience: generate + apply in one call
    # ------------------------------------------------------------------

    def rename(self, image_path: str) -> str:
        """
        Full pipeline: analyze image → generate name → apply rename.

        Args:
            image_path: Path to the image file.

        Returns:
            The new absolute path of the renamed file.
        """
        new_name = self.generate_new_name(image_path)
        return self.apply_rename(image_path, new_name)

    # ------------------------------------------------------------------
    # Dedicated Cleanup / Trash Check
    # ------------------------------------------------------------------
    
    def identify_junk(self, image_path: str, strict: bool = False) -> bool:
        """
        Hyper-specific check to evaluate if an image is pure cosmetic junk.
        Returns True if it's TRASH, False if KEEP.
        """
        if strict:
            system_p = STRICT_JUNK_PROMPT
        else:
            system_p = LAX_JUNK_PROMPT
        user_p = "Analyze this image and return TRASH or KEEP. Output nothing else."
        
        raw_response = self.provider.describe_image(image_path, user_p, system_prompt=system_p)
        return "TRASH" in raw_response.strip().upper()

    # ------------------------------------------------------------------
    # Lossless OCR / Markdown Extraction
    # ------------------------------------------------------------------
    
    def convert_to_markdown(self, image_path: str) -> str:
        """
        Evaluates an image. If it's text/tables/lists, transcribes it to markdown.
        Returns the markdown string prefixed with 'TEXT:' or exactly 'KEEP'.
        """
        system_p = MARKDOWN_CONVERSION_PROMPT
        user_p = "Extract to Markdown or Keep as Image? Respond strictly according to system rules."
        
        raw_response = self.provider.describe_image(image_path, user_p, system_prompt=system_p)
        raw_upper = raw_response.strip().upper()
        
        if raw_upper == "KEEP":
            return "KEEP"
        
        return raw_response.strip()