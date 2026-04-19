import threading
import queue
import time
from typing import Any, Dict, Generator
from pathlib import Path

from aicli.domains.analyze.database import AnalyzeDB
from aicli.cli.commands.analyze import _run_full_pipeline, _get_db


class SSEProgressContext:
    """A duck-typed rich.progress replacement that emits Server-Sent Events."""
    
    def __init__(self, event_queue: queue.Queue):
        self.queue = event_queue
        self.tasks = {}

    def add_task(self, description: str, total: int) -> int:
        task_id = len(self.tasks)
        self.tasks[task_id] = {"description": description, "total": total, "completed": 0}
        self.queue.put({
            "type": "task_add", 
            "task_id": task_id, 
            "description": description, 
            "total": total
        })
        return task_id

    def advance(self, task_id: int, advance: float = 1):
        if task_id in self.tasks:
            self.tasks[task_id]["completed"] += advance
            self.queue.put({
                "type": "task_progress",
                "task_id": task_id,
                "completed": self.tasks[task_id]["completed"],
                "total": self.tasks[task_id]["total"]
            })

    def stop(self):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        pass


class ConsoleRedirect:
    """Redirects rich console prints to the SSE queue."""
    def __init__(self, event_queue: queue.Queue):
        self.queue = event_queue
        
    def print(self, msg: str, *args, **kwargs):
        # We strip rich markup roughly or rely on frontend to handle basic ANSI
        self.queue.put({"type": "log", "message": str(msg)})


class AnalyzeOrchestrator:
    """Runs the analyze pipeline in a background thread and yields SSE."""
    
    _instance = None
    
    def __init__(self):
        self.queue = queue.Queue()
        self.is_running = False
        self.thread = None

    @classmethod
    def get_instance(cls):
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance

    def run_pipeline(self, data_dir: Path, workers: int, dpi: int, llm_model: str):
        if self.is_running:
            raise RuntimeError("Pipeline is already running.")
            
        self.is_running = True
        self.queue.queue.clear()
        
        self.thread = threading.Thread(
            target=self._worker, 
            args=(data_dir, workers, dpi, llm_model),
            daemon=True
        )
        self.thread.start()

    def _worker(self, data_dir: Path, workers: int, dpi: int, llm_model: str):
        self.queue.put({"type": "status", "status": "started"})
        
        # We need to monkey patch the functions in analyze to use our progress 
        # and console, but since Python relies on imports it might be tricky.
        import aicli.cli.commands.analyze as analyze_mod
        
        # Save originals
        orig_make_progress = analyze_mod._make_progress
        orig_console = analyze_mod.console
        orig_print_success = analyze_mod.print_success
        orig_print_error = analyze_mod.print_error
        
        try:
            db = _get_db(data_dir)
            
            # Monkeypatch
            analyze_mod._make_progress = lambda: SSEProgressContext(self.queue)
            analyze_mod.console = ConsoleRedirect(self.queue)
            
            def sse_success(msg):
                self.queue.put({"type": "log", "message": f"[SUCCESS] {msg}"})
            
            def sse_error(msg, exc):
                self.queue.put({"type": "log", "message": f"[ERROR] {msg} - {exc}"})
            
            analyze_mod.print_success = sse_success
            analyze_mod.print_error = sse_error
            
            # Run the pipeline
            _run_full_pipeline(
                data_dir=data_dir,
                db=db,
                workers=workers,
                dpi=dpi,
                pdf_files=None,
                llm_model=llm_model
            )
            
            self.queue.put({"type": "status", "status": "completed"})
            db.close()
            
        except Exception as e:
            self.queue.put({"type": "status", "status": "error", "message": str(e)})
        finally:
            self.is_running = False
            # Restore
            analyze_mod._make_progress = orig_make_progress
            analyze_mod.console = orig_console
            analyze_mod.print_success = orig_print_success
            analyze_mod.print_error = orig_print_error


    async def stream_events(self):
        """Async generator that yields SSE formatted strings without blocking event loop."""
        import asyncio
        while True:
            try:
                # Use non-blocking get, then sleep if empty
                event = self.queue.get_nowait()
                json_data = json.dumps(event)
                yield f"data: {json_data}\n\n"
                
                if event.get("type") == "status" and event.get("status") in ("completed", "error"):
                    break
            except queue.Empty:
                if not self.is_running:
                    break
                # Yield control to event loop
                await asyncio.sleep(0.5)
                # optionally yield ping to keep connection alive
                # yield "data: {\"type\": \"ping\"}\n\n"
