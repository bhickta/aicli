"""Lightweight HTTP API for the UPSC analyze pipeline viewer.

Serves JSON API endpoints reading from SQLite + page images from cache.
The Vue frontend (separate repo) connects to this.
"""
import json
import mimetypes
from http.server import HTTPServer, BaseHTTPRequestHandler
from pathlib import Path
from urllib.parse import parse_qs, unquote, urlparse

from aicli.domains.analyze.database import AnalyzeDB


class ViewerHandler(BaseHTTPRequestHandler):
    """HTTP handler serving JSON API + page images."""

    db: AnalyzeDB = None
    data_dir: Path = None
    cache_dir: Path = None

    def log_message(self, format, *args):
        pass

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        query = parse_qs(parsed.query)

        routes = {
            "/api/status": lambda: self._json(self.db.get_processing_status()),
            "/api/pdfs": lambda: self._json(self.db.get_all_pdfs()),
            "/api/aggregations": lambda: self._json(self._all_aggregations()),
        }

        if path in routes:
            routes[path]()
        elif path == "/api/pages":
            pdf = query.get("pdf", [None])[0]
            self._json(self.db.get_pages_for_pdf(pdf) if pdf else [])
        elif path == "/api/answers":
            pdf = query.get("pdf", [None])[0]
            self._json(self._answers_for_pdf(pdf) if pdf else [])
        elif path == "/api/dimensions":
            aid = query.get("answer_id", [None])[0]
            self._json(self._dims_for_answer(int(aid)) if aid else [])
        elif path.startswith("/images/"):
            self._serve_image(path[8:])  # strip /images/
        else:
            self.send_error(404)

    def do_POST(self):
        parsed = urlparse(self.path)
        path = parsed.path
        
        if path == "/api/reset":
            content_len = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(content_len)) if content_len else {}
            step = body.get("step", 2)
            self.db.reset_from_step(step)
            self._json({"ok": True, "reset_from_step": step})
        elif path == "/api/retry-errors":
            conn = self.db._get_conn()
            cur = conn.execute(
                "UPDATE pages SET transcription = NULL, processed = 0 "
                "WHERE transcription LIKE '[TRANSCRIPTION_ERROR%'"
            )
            conn.commit()
            self._json({"ok": True, "cleared": cur.rowcount})
        else:
            self.send_error(404)

    def do_OPTIONS(self):
        self.send_response(204)
        self._cors_headers()
        self.end_headers()

    def _cors_headers(self):
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type")

    def _json(self, data):
        body = json.dumps(data, ensure_ascii=False, default=str).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", len(body))
        self._cors_headers()
        self.end_headers()
        self.wfile.write(body)

    def _serve_image(self, rel_path: str):
        img_path = self.cache_dir / "images" / unquote(rel_path)
        if not img_path.exists():
            self.send_error(404)
            return
        mime, _ = mimetypes.guess_type(str(img_path))
        body = img_path.read_bytes()
        self.send_response(200)
        self.send_header("Content-Type", mime or "image/png")
        self.send_header("Content-Length", len(body))
        self.send_header("Cache-Control", "public, max-age=86400")
        self._cors_headers()
        self.end_headers()
        self.wfile.write(body)

    def _answers_for_pdf(self, pdf_file: str) -> list[dict]:
        conn = self.db._get_conn()
        rows = conn.execute(
            "SELECT * FROM answers WHERE pdf_file = ? ORDER BY question_number",
            (pdf_file,),
        ).fetchall()
        return [dict(r) for r in rows]

    def _dims_for_answer(self, answer_id: int) -> list[dict]:
        conn = self.db._get_conn()
        rows = conn.execute(
            "SELECT * FROM answer_dimensions WHERE answer_id = ? ORDER BY dimension_name",
            (answer_id,),
        ).fetchall()
        result = []
        for r in rows:
            d = dict(r)
            try:
                d["result_json"] = json.loads(d["result_json"])
            except (json.JSONDecodeError, TypeError):
                pass
            result.append(d)
        return result

    def _all_aggregations(self) -> list[dict]:
        aggs = self.db.get_all_aggregations()
        for a in aggs:
            try:
                a["aggregation_json"] = json.loads(a["aggregation_json"])
            except (json.JSONDecodeError, TypeError):
                pass
        return aggs


def start_viewer(data_dir: Path, port: int = 8765):
    """Start the viewer HTTP server. Returns the server instance."""
    db_path = data_dir / "analyze.db"
    cache_dir = data_dir / ".analyze_cache"

    if not db_path.exists():
        raise FileNotFoundError(f"No database found at {db_path}")

    db = AnalyzeDB(db_path)

    ViewerHandler.db = db
    ViewerHandler.data_dir = data_dir
    ViewerHandler.cache_dir = cache_dir

    server = HTTPServer(("0.0.0.0", port), ViewerHandler)
    return server
