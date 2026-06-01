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


def write_transcript_outputs(srt_path: Path, txt_path: Path, segments) -> None:
    srt_tmp = srt_path.with_suffix(srt_path.suffix + ".tmp")
    txt_tmp = txt_path.with_suffix(txt_path.suffix + ".tmp")
    with srt_tmp.open("w", encoding="utf-8") as srt_file, txt_tmp.open(
        "w", encoding="utf-8"
    ) as txt_file:
        index = 1
        for segment in segments:
            text = segment.text.strip()
            if not text:
                continue
            srt_file.write(f"{index}\n")
            srt_file.write(
                f"{format_srt_time(segment.start)} --> {format_srt_time(segment.end)}\n"
            )
            srt_file.write(f"{text}\n\n")
            txt_file.write(f"{text}\n")
            index += 1
    os.replace(srt_tmp, srt_path)
    os.replace(txt_tmp, txt_path)


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
        write_transcript_outputs(srt_path, txt_path, segments)

    with ThreadPoolExecutor(max_workers=max(1, args.workers)) as executor:
        futures = {executor.submit(transcribe, video): video for video in args.videos}
        for future in as_completed(futures):
            future.result()
            print(f"transcribed {futures[future]}", flush=True)

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
