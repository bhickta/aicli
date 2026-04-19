import typer
import uvicorn
from pathlib import Path

app = typer.Typer(help="Start the unified AICLI web backend API.")


def run_server(
    data_dir: Path,
    port: int = 8765,
    host: str = "0.0.0.0",
    workers: int = 1,
    cache_dir: Path = None,
    dev_mode: bool = False,
):
    import os
    if dev_mode:
        os.environ["AICLI_DEV_MODE"] = "1"
        
    from aicli.server.app import app as fastapi_app
    from aicli.server.routers.analyze import ServerState
    import subprocess
    import sys
    import signal
    
    # Inject directory context into the API
    ServerState.data_dir = data_dir.absolute()
    ServerState.cache_dir = cache_dir.absolute() if cache_dir else (data_dir / ".analyze_cache" / "images").absolute()
    ServerState.cache_dir.mkdir(parents=True, exist_ok=True)
    
    typer.echo(f"Starting AICLI API server on http://{host}:{port}")
    typer.echo(f"Active Data Directory: {data_dir.absolute()}")
    
    frontend_process = None
    if dev_mode:
        os.environ["AICLI_DEV_MODE"] = "1"
        frontend_dir = Path(__file__).parent.parent.parent.parent / "frontend"
        if frontend_dir.exists():
            typer.echo("🔥 DEV MODE DETECTED: Booting Vite Hot-Reload Server...")
            try:
                frontend_process = subprocess.Popen(
                    ["npm", "run", "dev"], 
                    cwd=str(frontend_dir),
                    preexec_fn=os.setsid
                )
                typer.echo("🔥 VITE STARTED: Open http://localhost:5173 for active hot-reloading!")
            except Exception as e:
                typer.echo(f"Failed to start Vite: {e}")
    
    try:
        uvicorn.run(fastapi_app, host=host, port=port)
    finally:
        if frontend_process:
            typer.echo("Shutting down Vite dev server...")
            try:
                os.killpg(os.getpgid(frontend_process.pid), signal.SIGTERM)
            except Exception:
                pass

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
    dev: bool = typer.Option(False, "--dev", is_flag=True, help="Enable Dev Mode with Vite HMR."),
):
    """Start the FastAPI backend server for the web UI."""
    run_server(data_dir=data_dir, port=port, host=host, dev_mode=dev)
