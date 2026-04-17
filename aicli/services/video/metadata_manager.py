import json
import base64
from pathlib import Path
from typing import Dict, Any

from aicli.services.video.ffmpeg_utils import FFprobeClient, FFmpegClient

class MetadataBackupManager:
    """Manages cache backups via sidecar during processing and native backup embedding."""
    
    @staticmethod
    def sidecar_path(video_path: Path) -> Path:
        cache_dir = video_path.parent / ".aicli_cache"
        cache_dir.mkdir(exist_ok=True, parents=True)
        new_sp = cache_dir / f"{video_path.stem}.sidecar.json"
        
        # Transparently migrate old root-level caches to the new hidden folder
        old_sp = video_path.with_suffix(".sidecar.json")
        if old_sp.exists() and not new_sp.exists():
            import shutil
            shutil.move(str(old_sp), str(new_sp))
            # Fallback to delete old if move creates a copy
            if old_sp.exists():
                old_sp.unlink()
                
        return new_sp

    @staticmethod
    def load_cache(video_path: Path) -> Dict[str, Any]:
        sp = MetadataBackupManager.sidecar_path(video_path)
        if sp.exists():
            try:
                return json.loads(sp.read_text(encoding="utf-8"))
            except Exception:
                pass
        return {}

    @staticmethod
    def save_cache(video_path: Path, data: Dict[str, Any]) -> None:
        sp = MetadataBackupManager.sidecar_path(video_path)
        sp.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")

    @staticmethod
    def backup_original_tags(video_path: Path, cache: Dict[str, Any]) -> tuple[Dict[str, Any], bool]:
        """Save original tags into sidecar once."""
        if "original_tags" not in cache:
            tags = FFprobeClient.read_existing_tags(video_path)
            cache["original_tags"] = tags
            return cache, True
        return cache, False

    @staticmethod
    def restore_original_tags(video_path: Path) -> bool:
        """Restore the video's original tags from embedded backup or sidecar."""
        # 1. Try embedded backup first
        existing = FFprobeClient.read_existing_tags(video_path)
        embedded_b64 = existing.get("aicli_backup", "")
        if not embedded_b64:
            # Fallback for uppercase format variations
            embedded_b64 = existing.get("AICLI_BACKUP", "")

        if embedded_b64:
            try:
                original = json.loads(base64.b64decode(embedded_b64).decode("utf-8"))
                return FFmpegClient.write_tags(video_path, original, clear_first=True)
            except Exception as e:
                raise ValueError(f"Found embedded backup but failed to parse: {e}")

        # 2. Fall back to sidecar
        sp = MetadataBackupManager.sidecar_path(video_path)
        if not sp.exists():
            raise FileNotFoundError(f"No embedded backup or sidecar found for {video_path.name}")
        
        cache = json.loads(sp.read_text(encoding="utf-8"))
        original = cache.get("original_tags")
        if not original:
            raise ValueError("No original_tags backup found in sidecar.")
            
        return FFmpegClient.write_tags(video_path, original, clear_first=True)

    @staticmethod
    def rename_cache_files(old_path: Path, new_path: Path) -> None:
        """Migrate all cache artifacts from old filename to new filename."""
        cache_dir = old_path.parent / ".aicli_cache"
        if not cache_dir.exists():
            return
        import shutil
        old_stem = old_path.stem
        new_stem = new_path.stem
        for f in cache_dir.iterdir():
            if f.name.startswith(old_stem):
                new_name = f.name.replace(old_stem, new_stem, 1)
                target = cache_dir / new_name
                if not target.exists():
                    shutil.move(str(f), str(target))

