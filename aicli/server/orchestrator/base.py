import threading
import queue
import json
from typing import Callable

class SSEProgressContext:
    """A duck-typed rich.progress replacement that emits Server-Sent Events."""
    def __init__(self, event_queue: queue.Queue):
        self.queue = event_queue
        self.tasks = {}
        self.console = ConsoleRedirect(event_queue)

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


import re

class ConsoleRedirect:
    """Redirects rich console prints to the SSE queue."""
    def __init__(self, event_queue: queue.Queue):
        self.queue = event_queue
        # Regex to strip [bold magenta], [red], [dim], etc.
        self.tag_re = re.compile(r"\[/?(?:[a-z ]+|#[0-9a-f]{6})\]")
        
    def print(self, msg: str, *args, **kwargs):
        # Convert to string and strip Rich tags
        clean_msg = self.tag_re.sub("", str(msg))
        self.queue.put({"type": "log", "message": clean_msg})


class BaseOrchestrator:
    """Runs a pipeline in a background thread and yields SSE."""
    
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

    def dispatch(self, worker_target: Callable, *args, **kwargs):
        """Starts the pipeline thread."""
        if self.is_running:
            raise RuntimeError("Pipeline is already running.")
            
        self.is_running = True
        self.queue.queue.clear()
        
        self.thread = threading.Thread(
            target=self._run_wrapper, 
            args=(worker_target, *args),
            kwargs=kwargs,
            daemon=True
        )
        self.thread.start()

    def _run_wrapper(self, worker_target: Callable, *args, **kwargs):
        self.queue.put({"type": "status", "status": "started"})
        try:
            worker_target(self, *args, **kwargs)
            self.queue.put({"type": "status", "status": "completed"})
        except Exception as e:
            self.queue.put({"type": "status", "status": "error", "message": str(e)})
        finally:
            self.is_running = False

    async def stream_events(self):
        """Async generator that yields SSE formatted strings without blocking event loop."""
        import asyncio
        while True:
            try:
                event = self.queue.get_nowait()
                json_data = json.dumps(event)
                yield {"data": json_data}
                
                if event.get("type") == "status" and event.get("status") in ("completed", "error"):
                    break
            except queue.Empty:
                # If we've just connected and the orchestrator isn't running yet,
                # wait a bit before giving up, as the 'run' POST might be arriving.
                if not self.is_running:
                    # Small grace period (e.g. 5 seconds) could be handled with a counter
                    # but for now, we'll just check if it's been empty for a while.
                    # As a simpler fix: don't break immediately if we've just started.
                    await asyncio.sleep(1.0)
                    if not self.is_running:
                        break
                await asyncio.sleep(0.5)
