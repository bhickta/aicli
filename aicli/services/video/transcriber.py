from pathlib import Path
from typing import Dict, Any, List

from aicli.services.video.ffmpeg_utils import FFprobeClient, FFmpegClient
from aicli.config import config as app_config

class WhisperEngine:
    """Manages speech-to-text generation using faster-whisper."""
    
    @staticmethod
    def load_whisper(model_size: str, num_workers: int = 1):
        try:
            from faster_whisper import WhisperModel, BatchedInferencePipeline
        except ImportError:
            raise ImportError("faster-whisper is not installed. Run: uv add faster-whisper numpy")
        
        # num_workers is kept for CTranslate2 internal thread pools but real parallelism
        # comes from batch_size passed to .transcribe() calls
        model = WhisperModel(model_size, device="cuda", compute_type="float16", num_workers=num_workers)
        return BatchedInferencePipeline(model=model)

    @staticmethod
    def compute_clip_times(duration: float, clip_every: int, clip_len: int) -> List[float]:
        if duration < clip_len:
            return [0]
        margin = duration * 0.05
        times, t = [], margin
        while t <= duration - margin - clip_len:
            times.append(t)
            t += clip_every
        return times

    @staticmethod
    def transcribe_video_sparse(video_path: Path, whisper_model, clip_every: int, clip_len: int) -> List[Dict[str, Any]]:
        duration = FFprobeClient.get_duration(video_path)
        times = WhisperEngine.compute_clip_times(duration, clip_every, clip_len)
        results = []
        
        for t in times:
            audio = FFmpegClient.stream_audio_clip(video_path, t, clip_len)
            if audio is None or len(audio) < 1600: # ~0.1s minimum audio
                continue
                
            segments, _ = whisper_model.transcribe(
                audio, 
                batch_size=app_config.whisper_batch_size, 
                beam_size=app_config.whisper_beam_size, 
                language=None, 
                vad_filter=True
            )
            text = " ".join(s.text.strip() for s in segments).strip()
            
            if text:
                results.append({"start_sec": round(t, 1), "text": text})
                    
        return results

    @staticmethod
    def _format_srt_time(seconds: float) -> str:
        hours = int(seconds // 3600)
        minutes = int((seconds % 3600) // 60)
        seconds_r = seconds % 60
        milliseconds = int((seconds_r - int(seconds_r)) * 1000)
        return f"{hours:02}:{minutes:02}:{int(seconds_r):02},{milliseconds:03}"

    @staticmethod
    def transcribe_video_full_srt(video_path: Path, whisper_model, srt_path: Path) -> List[Dict[str, Any]]:
        """Fully transcribes the video, writes it to SRT, and returns sparse clips for LM Studio."""
        segments, _ = whisper_model.transcribe(
            str(video_path), 
            batch_size=app_config.whisper_batch_size, 
            beam_size=app_config.whisper_beam_size, 
            language=None, 
            vad_filter=True
        )
        
        sample_clips = []
        # Store segments into a list since generators are consumed
        seg_list = list(segments)
        
        # Write SRT
        with srt_path.open("w", encoding="utf-8") as f:
            for i, segment in enumerate(seg_list, start=1):
                f.write(f"{i}\n")
                f.write(f"{WhisperEngine._format_srt_time(segment.start)} --> {WhisperEngine._format_srt_time(segment.end)}\n")
                f.write(f"{segment.text.strip()}\n\n")

        # Extract 5 evenly spaced segments for LM Studio sparse context (to not blow up context window)
        if seg_list:
            step = max(1, len(seg_list) // 5)
            for s in seg_list[::step][:5]:
                sample_clips.append({"start_sec": s.start, "text": s.text.strip()})
                
        return sample_clips

    @staticmethod
    def extract_clips_from_existing_srt(srt_path: Path) -> List[Dict[str, Any]]:
        """Parse an existing SRT file strictly to pull text snippets for LM Studio context."""
        import re
        sample_clips = []
        try:
            content = srt_path.read_text(encoding="utf-8").strip().split('\n\n')
            blocks = []
            for block in content:
                lines = block.split('\n')
                if len(lines) >= 3:
                    # '00:01:23,450 --> 00:01:25,000'
                    time_line = lines[1]
                    m = re.search(r'(\d+):(\d+):(\d+)', time_line)
                    if m:
                        sec = int(m.group(1))*3600 + int(m.group(2))*60 + int(m.group(3))
                        text = " ".join(lines[2:])
                        blocks.append({"start_sec": sec, "text": text})

            if blocks:
                step = max(1, len(blocks) // 5)
                sample_clips = blocks[::step][:5]
        except Exception:
            pass
        return sample_clips
