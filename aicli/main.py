import typer
from aicli.cli.commands import image, pdf, news, video

app = typer.Typer(
    help="AI CLI - Power up your terminal with local AI (LM Studio)."
)

# Register command subgroups
app.add_typer(image.app, name="image")
app.add_typer(pdf.app, name="pdf")
app.add_typer(news.app, name="news")
app.add_typer(video.app, name="video")

def run():
    """Entry point for the CLI."""
    app()

if __name__ == "__main__":
    run()
