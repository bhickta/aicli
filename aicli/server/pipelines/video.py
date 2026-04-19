"""Video command group — Routes to focused subcommand modules."""
import typer

from aicli.cli.commands import video_tag, video_info, video_notes, video_compress, video_course

app = typer.Typer(help="Video transcription and metadata tagging commands.")

# Register all subcommands from their focused modules
video_tag.register(app)
video_info.register(app)
video_notes.register(app)
video_compress.register(app)
video_course.register(app)
