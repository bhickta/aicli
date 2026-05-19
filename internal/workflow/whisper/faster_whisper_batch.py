import argparse
import os
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

from faster_whisper import BatchedInferencePipeline, WhisperModel


def format_srt_time(seconds: float) -> str:
    hours = int(seconds // 3600)
    minutes = int((seconds % 3600) // 60)
    secs = seconds % 60
    millis = int((secs - int(secs)) * 1000)
    return f"{hours:02}:{minutes:02}:{int(secs):02},{millis:03}"


def atomic_write(path: Path, content: str) -> None:
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(content, encoding="utf-8")
    os.replace(tmp, path)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", default="large-v3")
    parser.add_argument("--device", default="cuda")
    parser.add_argument("--compute-type", default="float16")
    parser.add_argument("--workers", type=int, default=2)
    parser.add_argument("--batch-size", type=int, default=24)
    parser.add_argument("--beam-size", type=int, default=1)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("videos", nargs="+")
    args = parser.parse_args()

    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    model = WhisperModel(
        args.model,
        device=args.device,
        compute_type=args.compute_type,
        num_workers=max(1, args.workers),
    )
    pipeline = BatchedInferencePipeline(model=model)

    def transcribe(video: str) -> None:
        video_path = Path(video)
        srt_path = output_dir / f"{video_path.stem}.srt"
        txt_path = output_dir / f"{video_path.stem}.txt"
        if srt_path.exists() and txt_path.exists():
            return

        segments, _ = pipeline.transcribe(
            str(video_path),
            batch_size=args.batch_size,
            beam_size=args.beam_size,
            language=None,
            vad_filter=True,
        )
        segment_list = list(segments)

        srt_lines = []
        txt_lines = []
        for index, segment in enumerate(segment_list, start=1):
            text = segment.text.strip()
            if not text:
                continue
            srt_lines.append(str(index))
            srt_lines.append(
                f"{format_srt_time(segment.start)} --> {format_srt_time(segment.end)}"
            )
            srt_lines.append(text)
            srt_lines.append("")
            txt_lines.append(text)

        atomic_write(srt_path, "\n".join(srt_lines).strip() + "\n")
        atomic_write(txt_path, "\n".join(txt_lines).strip() + "\n")

    with ThreadPoolExecutor(max_workers=max(1, args.workers)) as executor:
        futures = {executor.submit(transcribe, video): video for video in args.videos}
        for future in as_completed(futures):
            future.result()
            print(f"transcribed {futures[future]}", flush=True)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
