import os
import re
from pathlib import Path
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
    def _build_system_prompt(trash_junk: bool = False) -> str:
        prompt = (
            "You are a precise image archiving system. Your sole function is to output a single "
            "kebab-case filename string that captures the most identifying characteristics of an image.\n\n"

            "ANALYSIS HIERARCHY (evaluate in this order):\n"
            "1. PROMINENT TEXT — If the image contains large, clear text (signs, labels, titles, logos, "
            "watermarks, banners), that text takes absolute priority and must anchor the filename.\n"
            "2. SUBJECT & ACTION — The primary subject (person, object, animal, vehicle) and what it is "
            "doing or its defining state (e.g. 'dog-catching-frisbee', 'broken-screen-iphone').\n"
            "3. CONTEXT & SETTING — Location, environment, or scene type only if it meaningfully "
            "distinguishes the image (e.g. 'aerial-view-manhattan', 'underwater-coral-reef').\n"
            "4. DOMINANT COLOR OR STYLE — Include only if it is the most distinguishing trait "
            "(e.g. 'neon-pink-graffiti-wall', 'vintage-sepia-portrait').\n\n"

            "OUTPUT RULES — STRICTLY ENFORCED:\n"
            f"- Output the raw kebab-case string and NOTHING else.\n"
            f"- {ImageRenamerService.MIN_WORDS} to {ImageRenamerService.MAX_WORDS} words. "
            f"Never fewer than {ImageRenamerService.MIN_WORDS}, never more than {ImageRenamerService.MAX_WORDS}.\n"
            "- Lowercase letters, digits, and hyphens ONLY. No spaces, underscores, punctuation, or extensions.\n"
            "- No filler words: no 'a', 'an', 'the', 'of', 'with', 'in', 'on'.\n"
            "- No meta-commentary: never output 'image-of', 'photo-of', 'picture-of', 'screenshot-of'.\n"
            "- No uncertainty markers: never output 'unknown', 'unclear', 'possible', 'maybe'.\n"
            "- Be SPECIFIC over generic: 'golden-retriever-muddy-paws' beats 'dog-outside'.\n\n"

            "FORBIDDEN OUTPUTS:\n"
            "- Sentences or phrases in natural language.\n"
            "- Markdown formatting, backticks, quotes, or brackets.\n"
            "- Any explanation of your reasoning.\n\n"

            "VALID EXAMPLES:\n"
            "  vintage-red-sports-car\n"
            "  google-login-error-screen\n"
            "  snow-covered-mountain-peak\n"
            "  smiling-child-birthday-cake\n"
            "  amazon-prime-invoice-2024\n\n"

            "INVALID EXAMPLES (never do this):\n"
            "  'Here is the filename: red-car'   <- contains prose\n"
            "  image-of-a-dog                    <- filler words + meta prefix\n"
            "  red_sports_car.jpg                <- underscores + extension\n"
            "  car                               <- too vague, too short\n"
        )
        
        if trash_junk:
            prompt += (
                "\nSPECIAL JUNK FILTER:\n"
                "If the image is purely a cosmetic website icon, corporate logo, watermark, UI element, barcode, "
                "or decorative graphic that provides absolutely ZERO useful study/informational value, "
                "you MUST output EXACTLY the word 'TRASH' instead of generating a filename."
            )
            
        return prompt

    @staticmethod
    def _build_user_prompt() -> str:
        return (
            "Examine this image carefully. Apply the analysis hierarchy: check for prominent text first, "
            "then identify the main subject, action, setting, and distinguishing visual traits. "
            f"Synthesize the most identifying {ImageRenamerService.MIN_WORDS}–{ImageRenamerService.MAX_WORDS} "
            "word kebab-case filename. "
            "Output ONLY that string — no other characters."
        )

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
        raw_response = self.provider.describe_image(
            image_path,
            self._build_user_prompt(),
            system_prompt=self._build_system_prompt(trash_junk),
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

        # Fallback: join last MIN_WORDS meaningful words into a slug
        content_words = [
            w.lower() for w in words
            if w.lower() not in {"a", "an", "the", "of", "with", "in", "on", "is", "are"}
        ]
        slug_words = content_words[-ImageRenamerService.MIN_WORDS:]
        return "-".join(slug_words) if slug_words else text

    @staticmethod
    def _enforce_word_count(slug: str) -> str:
        """
        Ensures the slug falls within [MIN_WORDS, MAX_WORDS] hyphen-separated tokens.
        Truncates if too long; returns as-is if within bounds or too short
        (too-short slugs may still be meaningful, e.g. a two-word brand name).
        """
        parts = slug.split("-")
        if len(parts) > ImageRenamerService.MAX_WORDS:
            parts = parts[:ImageRenamerService.MAX_WORDS]
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
    
    def identify_junk(self, image_path: str) -> bool:
        """
        Hyper-specific check to evaluate if an image is pure cosmetic junk.
        Returns True if it's TRASH, False if KEEP.
        """
        system_p = (
            "You are a strict QA filter for a study database. Your ONLY job is to classify the attached image.\n"
            "If the image is a generic website icon, corporate logo, watermark, UI element, barcode, or tiny decorative graphic that provides absolutely ZERO useful study/informational value, output EXACTLY the word 'TRASH'.\n"
            "If the image contains ANY useful information (even if it's a map, diagram, readable text block, or photo), output EXACTLY the word 'KEEP'."
        )
        user_p = "Analyze this image and return TRASH or KEEP. Output nothing else."
        
        raw_response = self.provider.describe_image(image_path, user_p, system_prompt=system_p)
        return "TRASH" in raw_response.strip().upper()