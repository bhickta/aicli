import typer
import uvicorn
from pathlib import Path

app = typer.Typer(help="Start the unified AICLI web backend API.")

@app.callback(invoke_without_command=True)
def main(
    data_dir: Path = typer.Option(
        ".",
        "--data-dir", "-d",
        exists=True,
        file_okay=False,
        dir_okay=True,
        help="Data directory for the active session.",
    ),
    port: int = typer.Option(8765, "--port", "-p", help="Port to serve on."),
    host: str = typer.Option("0.0.0.0", "--host", help="Host interface to bind to."),
):
    """Start the FastAPI backend server for the web UI."""
    from aicli.server.app import app as fastapi_app
    from aicli.server.routers.analyze import ServerState
    
    # Inject directory context into the API
    ServerState.data_dir = data_dir.absolute()
    ServerState.cache_dir = (data_dir / ".analyze_cache" / "images").absolute()
    ServerState.cache_dir.mkdir(parents=True, exist_ok=True)
    
    typer.echo(f"Starting AICLI API server on http://{host}:{port}")
    typer.echo(f"Active Data Directory: {data_dir.absolute()}")
    
    uvicorn.run(fastapi_app, host=host, port=port)
