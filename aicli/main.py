import typer
from aicli.cli.commands import image, pdf, news, video, analyze

app = typer.Typer(
    help="AI CLI - Power up your terminal with local AI (LM Studio)."
)

# Register command subgroups
app.add_typer(image.app, name="image")
app.add_typer(pdf.app, name="pdf")
app.add_typer(news.app, name="news")
app.add_typer(video.app, name="video")
app.add_typer(analyze.app, name="analyze")

def run():
    """Entry point for the CLI."""
    app()

if __name__ == "__main__":
    run()
