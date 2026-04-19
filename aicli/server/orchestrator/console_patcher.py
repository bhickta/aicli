"""Reusable context manager for monkey-patching console output to SSE queues."""
import queue
from types import ModuleType
from typing import Optional

from aicli.server.orchestrator.base import ConsoleRedirect


class ConsolePatcher:
    """Context manager that redirects a module's console/print functions to an SSE queue.

    Saves original attributes on enter and restores them on exit.
    Eliminates the duplicated monkey-patch boilerplate across all router workers.

    Usage:
        with ConsolePatcher(some_module, orch.queue):
            some_module.do_work()
    """

    _PATCHABLE_ATTRS = ("console", "print_success", "print_error")

    def __init__(self, module: ModuleType, event_queue: queue.Queue) -> None:
        self._module = module
        self._queue = event_queue
        self._originals: dict[str, Optional[object]] = {}

    def __enter__(self) -> "ConsolePatcher":
        self._save_originals()
        self._apply_patches()
        return self

    def __exit__(self, exc_type: type, exc_val: Exception, exc_tb: object) -> None:
        self._restore_originals()

    def _save_originals(self) -> None:
        for attr in self._PATCHABLE_ATTRS:
            self._originals[attr] = getattr(self._module, attr, None)

    def _apply_patches(self) -> None:
        if hasattr(self._module, "console"):
            self._module.console = ConsoleRedirect(self._queue)
        if hasattr(self._module, "print_success"):
            self._module.print_success = self._make_log_fn("[SUCCESS]")
        if hasattr(self._module, "print_error"):
            self._module.print_error = self._make_error_fn()

    def _restore_originals(self) -> None:
        for attr, original in self._originals.items():
            if original is not None:
                setattr(self._module, attr, original)

    def _make_log_fn(self, prefix: str):
        """Create a logging function that pushes prefixed messages to the queue."""
        q = self._queue

        def _log(msg: str) -> None:
            q.put({"type": "log", "message": f"{prefix} {msg}"})

        return _log

    def _make_error_fn(self):
        """Create an error logging function compatible with the (msg, exc=None) signature."""
        q = self._queue

        def _error(msg: str, exc: Exception = None) -> None:
            q.put({"type": "log", "message": f"[ERROR] {msg} {exc or ''}"})

        return _error
