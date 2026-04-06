import typer
from aicli.cli.commands import image

app = typer.Typer(
    help="AI CLI - Power up your terminal with local AI (LM Studio)."
)

# Register command subgroups
app.add_typer(image.app, name="image")

def run():
    """Entry point for the CLI."""
    app()

if __name__ == "__main__":
    run()
