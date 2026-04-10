from pathlib import Path
from typing import Dict, Any, List

from aicli.services.video.ffmpeg_utils import FFprobeClient, FFmpegClient

class WhisperEngine:
    """Manages speech-to-text generation using faster-whisper."""
    
    @staticmethod
    def load_whisper(model_size: str):
        try:
            from faster_whisper import WhisperModel
        except ImportError:
            raise ImportError("faster-whisper is not installed. Run: uv add faster-whisper numpy")
        return WhisperModel(model_size, device="cuda", compute_type="float16")

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
    def transcribe_video(video_path: Path, whisper_model, clip_every: int, clip_len: int) -> List[Dict[str, Any]]:
        duration = FFprobeClient.get_duration(video_path)
        times = WhisperEngine.compute_clip_times(duration, clip_every, clip_len)
        results = []
        
        for t in times:
            audio = FFmpegClient.stream_audio_clip(video_path, t, clip_len)
            if audio is None or len(audio) < 1600: # ~0.1s minimum audio
                continue
                
            segments, _ = whisper_model.transcribe(
                audio, beam_size=5, language=None, vad_filter=True
            )
            text = " ".join(s.text.strip() for s in segments).strip()
            
            if text:
                results.append({"start_sec": round(t, 1), "text": text})
                    
        return results
