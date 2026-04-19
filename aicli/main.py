import typer
from aicli.cli.commands import server

app = typer.Typer(
    help="AI CLI - The Unified Control Center API and UI Platform Server."
)

# Register command subgroups
app.add_typer(server.app, name="server", help="Start the Unified God-Mode Web UI Server")

# Alias for 'serve' for backwards compatibility
@app.command()
def serve(
    host: str = "0.0.0.0",
    port: int = 8765,
    workers: int = 1,
    data_dir: str = "./data",
    cache_dir: str = "./.aicli_cache"
):
    """Alias for 'aicli server'."""
    from aicli.cli.commands.server import run_server
    from pathlib import Path
    run_server(host=host, port=port, workers=workers, data_dir=Path(data_dir), cache_dir=Path(cache_dir))

def run():
    """Entry point for the CLI."""
    app()

if __name__ == "__main__":
    run()
