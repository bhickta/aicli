"""Base orchestrator for running pipelines in background threads with SSE streaming."""
import asyncio
import json
import queue
import re
import threading
from typing import Any, AsyncGenerator, Callable, Dict


class PipelineAbortedError(BaseException):
    """Raised when the pipeline is requested to stop by the user."""
    pass


class ConsoleRedirect:
    """Redirects rich console prints to an SSE queue, stripping markup tags."""

    _TAG_RE = re.compile(r"\[/?(?:[a-z ]+|#[0-9a-f]{6})\]")

    def __init__(self, event_queue: queue.Queue, abort_event: threading.Event | None = None) -> None:
        self.queue = event_queue
        self.abort_event = abort_event

    def print(self, msg: str, *args: Any, **kwargs: Any) -> None:
        if self.abort_event and self.abort_event.is_set():
            raise PipelineAbortedError("Pipeline aborted by user")
        clean_msg = self._TAG_RE.sub("", str(msg))
        self.queue.put({"type": "log", "message": clean_msg})


class SSEProgressContext:
    """A duck-typed rich.progress replacement that emits Server-Sent Events."""

    def __init__(self, event_queue: queue.Queue, abort_event: threading.Event | None = None) -> None:
        self.queue = event_queue
        self.abort_event = abort_event
        self.tasks: Dict[int, Dict[str, Any]] = {}
        self.console = ConsoleRedirect(event_queue, abort_event)

    def add_task(self, description: str, total: int) -> int:
        if self.abort_event and self.abort_event.is_set():
            raise PipelineAbortedError("Pipeline aborted by user")
        task_id = len(self.tasks)
        self.tasks[task_id] = {"description": description, "total": total, "completed": 0}
        self.queue.put({
            "type": "task_add",
            "task_id": task_id,
            "description": description,
            "total": total,
        })
        return task_id

    def advance(self, task_id: int, advance: float = 1) -> None:
        if self.abort_event and self.abort_event.is_set():
            raise PipelineAbortedError("Pipeline aborted by user")
        if task_id not in self.tasks:
            return
        self.tasks[task_id]["completed"] += advance
        self.queue.put({
            "type": "task_progress",
            "task_id": task_id,
            "completed": self.tasks[task_id]["completed"],
            "total": self.tasks[task_id]["total"],
        })

    def stop(self) -> None:
        pass

    def __enter__(self) -> "SSEProgressContext":
        return self

    def __exit__(self, exc_type: type, exc_val: Exception, exc_tb: Any) -> None:
        pass


class BaseOrchestrator:
    """Runs a pipeline in a background thread and yields SSE events."""

    def __init__(self) -> None:
        self.queue: queue.Queue = queue.Queue()
        self.is_running: bool = False
        self.thread: threading.Thread | None = None
        self.abort_event: threading.Event = threading.Event()

    def abort(self) -> None:
        if self.is_running:
            self.abort_event.set()

    def dispatch(self, worker_target: Callable, *args: Any, **kwargs: Any) -> None:
        """Start the pipeline thread."""
        if self.is_running:
            raise RuntimeError("Pipeline is already running.")

        self.is_running = True
        self.abort_event.clear()
        self.queue.queue.clear()

        self.thread = threading.Thread(
            target=self._run_wrapper,
            args=(worker_target, *args),
            kwargs=kwargs,
            daemon=True,
        )
        self.thread.start()

    async def stream_events(self) -> AsyncGenerator[Dict[str, str], None]:
        """Async generator that yields SSE-formatted dicts without blocking the event loop."""
        while True:
            try:
                event = self.queue.get_nowait()
                yield {"data": json.dumps(event)}
                if self._is_terminal_event(event):
                    break
            except queue.Empty:
                if not self.is_running:
                    await asyncio.sleep(1.0)
                    if not self.is_running:
                        break
                await asyncio.sleep(0.5)

    # ── Private ─────────────────────────────────────────────────────

    def _run_wrapper(self, worker_target: Callable, *args: Any, **kwargs: Any) -> None:
        self.queue.put({"type": "status", "status": "started"})
        try:
            worker_target(self, *args, **kwargs)
            self.queue.put({"type": "status", "status": "completed"})
        except Exception as e:
            self.queue.put({"type": "status", "status": "error", "message": str(e)})
        except BaseException as e:
            if type(e).__name__ == "PipelineAbortedError":
                self.queue.put({"type": "status", "status": "error", "message": "Pipeline aborted by user."})
            else:
                raise
        finally:
            self.is_running = False

    @staticmethod
    def _is_terminal_event(event: Dict[str, Any]) -> bool:
        return event.get("type") == "status" and event.get("status") in ("completed", "error")
