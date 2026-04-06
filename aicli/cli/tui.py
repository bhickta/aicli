from rich.console import Console
from rich.panel import Panel
from rich.text import Text
import typer

# Global rich console
console = Console()

def print_header(title: str):
    """Print a beautiful header for a command."""
    panel = Panel(Text(title, justify="center", style="bold cyan"))
    console.print(panel)

def print_success(message: str):
    """Print a success message."""
    console.print(f"[bold green]✔[/bold green] {message}")

def print_error(message: str, error: Exception = None):
    """Print an error message."""
    text = f"[bold red]✖[/bold red] {message}"
    if error:
        text += f": {str(error)}"
    console.print(text)
    
def confirm_action(message: str) -> bool:
    """Prompt the user for confirmation."""
    return typer.confirm(message)
