#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_URL="${AICLI_BASE_URL:-http://127.0.0.1:8765}"
SOURCE_DIR="${AICLI_COURSE_SOURCE:-/home/bhickta/philosophy_tanu_jain}"
WORK_DIR="${AICLI_COURSE_WORKDIR:-/home/bhickta/.config/aicli/course-work/philosophy_tanu_jain}"
OUTPUT_NAME="${AICLI_COURSE_OUTPUT_NAME:-philosophy_tanu_jain}"
LOG_DIR="${AICLI_COURSE_LOG_DIR:-/home/bhickta/.config/aicli/course-runs}"
STAMP="$(date +%Y%m%d-%H%M%S)"
SERVER_LOG="$LOG_DIR/aicli-server-$STAMP.log"
RUN_LOG="$LOG_DIR/philosophy-course-$STAMP.log"

mkdir -p "$LOG_DIR"

cd "$ROOT_DIR"

server_started=0
if ! curl -fsS "$BASE_URL/api/health" >/dev/null 2>&1; then
  echo "Starting AICLI server..." | tee -a "$RUN_LOG"
  go run ./cmd/aicli >"$SERVER_LOG" 2>&1 &
  server_started=1
  for _ in $(seq 1 30); do
    if curl -fsS "$BASE_URL/api/health" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
fi

if ! curl -fsS "$BASE_URL/api/health" >/dev/null 2>&1; then
  echo "AICLI server is not healthy. Server log: $SERVER_LOG" | tee -a "$RUN_LOG"
  exit 1
fi

python3 - "$BASE_URL" "$SOURCE_DIR" "$WORK_DIR" "$OUTPUT_NAME" "$RUN_LOG" <<'PY'
import json
import shutil
import subprocess
import sys
import time
from pathlib import Path
from urllib import request

base_url = sys.argv[1].rstrip("/")
source_dir = Path(sys.argv[2])
work_dir = Path(sys.argv[3])
output_name = sys.argv[4]
run_log = Path(sys.argv[5])

def log(message):
    line = f"{time.strftime('%F %T')} {message}"
    print(line, flush=True)
    with run_log.open("a", encoding="utf-8") as handle:
        handle.write(line + "\n")

def api(path, method="GET", body=None):
    data = json.dumps(body).encode("utf-8") if body is not None else None
    headers = {"Content-Type": "application/json"} if body is not None else {}
    req = request.Request(base_url + path, data=data, headers=headers, method=method)
    with request.urlopen(req, timeout=30) as response:
        raw = response.read().decode("utf-8")
    return json.loads(raw or "{}")

if not source_dir.exists():
    raise SystemExit(f"source folder does not exist: {source_dir}")

payload = {
    "path": str(source_dir),
    "work_dir": str(work_dir),
    "output_name": output_name,
    "whisper_model": "large-v3",
    "whisper_device": "cuda",
    "preset": "slideshow",
    "resolution": 0,
    "crf": 0,
    "fps": "1/2",
    "fast_skip": True,
    "transcript_workers": 3,
    "compression_workers": 4,
    "skip_unreadable": False,
    "cleanup_verified_parts": False,
    "max_merge_hours": 9,
}

job = api("/api/workflows/video/course", "POST", payload)["job"]
job_id = job["id"]
log(f"started job={job_id}")

while True:
    job = api(f"/api/jobs/{job_id}")
    status = job.get("status", "")
    progress = float(job.get("progress", 0) or 0)
    stage = job.get("stage", "")
    log(f"job={job_id} status={status} progress={progress:.1%} stage={stage}")

    if status == "completed":
        break
    if status in ("failed", "cancelled"):
        log("job did not complete; NOT moving source files and NOT shutting down")
        error = job.get("error", "")
        if error:
            log(f"error={error}")
        raise SystemExit(1)
    time.sleep(30)

stamp = time.strftime("%Y%m%d-%H%M%S")
trash_dir = source_dir / "trash" / f"aicli-course-success-{stamp}"
trash_dir.mkdir(parents=True, exist_ok=True)

video_exts = {".mp4", ".mov", ".mkv", ".webm", ".avi", ".m4v"}
moved = 0

for path in sorted(source_dir.rglob("*")):
    if not path.is_file():
        continue
    rel_parts = path.relative_to(source_dir).parts
    if "Course" in rel_parts or "trash" in rel_parts or ".aicli_cache" in rel_parts:
        continue
    if path.suffix.lower() not in video_exts:
        continue

    for item in (path, path.with_suffix(".srt")):
        if not item.exists():
            continue
        dest = trash_dir / item.relative_to(source_dir)
        dest.parent.mkdir(parents=True, exist_ok=True)
        if dest.exists():
            dest = dest.with_name(f"{dest.stem}-{stamp}{dest.suffix}")
        shutil.move(str(item), str(dest))
        moved += 1

if work_dir.exists():
    shutil.rmtree(work_dir)

log(f"moved {moved} source/sidecar files to {trash_dir}")
log("course completed successfully; powering off")
subprocess.run(["systemctl", "poweroff"], check=False)
PY

if [[ "$server_started" == "1" ]]; then
  echo "AICLI server was started by this script. Log: $SERVER_LOG" >>"$RUN_LOG"
fi
