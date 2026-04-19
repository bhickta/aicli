from pathlib import Path
import re

fpath = Path("aicli/server/pipelines/video_notes.py")
content = fpath.read_text()

orig = """def register(app: typer.Typer):
    \"\"\"Register the notes command on the given Typer app.\"\"\"

    @app.command("notes")
    def generate_notes(
        target_path: Path = typer.Argument(
            ...,
            exists=True,
            help="Path to a video file or directory. Processes videos with sidecar .srt files OR embedded subtitle streams."
        ),
        overwrite: bool = typer.Option(
            False,
            "--overwrite",
            help="Overwrite existing .md notes files."
        ),
        style: str = typer.Option(
            "bullet",
            "--style", "-s",
            help="Note style: 'bullet' (ultra-dense, default) or 'clean' (removes fluff, no info loss)."
        ),
    ):
        \"\"\"
        Generate ultra-dense exam-ready study notes from SRT transcripts via LM Studio.

        Detects subtitles from sidecar .srt files or embedded subtitle streams inside the
        video container. Converts to plain text, sends to LM Studio, saves as .md file.
        \"\"\""""

new = """def process_notes(target_path: Path, overwrite: bool = False, style: str = "bullet"):
    \"\"\"Core logic to generate video notes, extracted for API usage.\"\"\"

def register(app: typer.Typer):
    \"\"\"Register the notes command on the given Typer app.\"\"\"

    @app.command("notes")
    def generate_notes(
        target_path: Path = typer.Argument(
            ...,
            exists=True,
            help="Path to a video file or directory. Processes videos with sidecar .srt files OR embedded subtitle streams."
        ),
        overwrite: bool = typer.Option(
            False,
            "--overwrite",
            help="Overwrite existing .md notes files."
        ),
        style: str = typer.Option(
            "bullet",
            "--style", "-s",
            help="Note style: 'bullet' (ultra-dense, default) or 'clean' (removes fluff, no info loss)."
        ),
    ):
        \"\"\"
        Generate ultra-dense exam-ready study notes from SRT transcripts via LM Studio.

        Detects subtitles from sidecar .srt files or embedded subtitle streams inside the
        video container. Converts to plain text, sends to LM Studio, saves as .md file.
        \"\"\"
        process_notes(target_path, overwrite, style)

def process_notes(target_path: Path, overwrite: bool = False, style: str = "bullet"):"""

content = content.replace(orig, new)
fpath.write_text(content)
print("Patched notes wrapper.")
