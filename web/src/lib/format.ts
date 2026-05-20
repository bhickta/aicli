export function elapsedSeconds(value: string): number {
  const started = Date.parse(value);
  if (!Number.isFinite(started)) return 0;
  return Math.max(0, Math.round((Date.now() - started) / 1000));
}

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return "0s";
  const rounded = Math.round(seconds);
  const h = Math.floor(rounded / 3600);
  const m = Math.floor((rounded % 3600) / 60);
  const s = rounded % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function readNumberValue(value: unknown, fallback: number, min = 0): number {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return fallback;
  return Math.max(min, parsed);
}

export function stringify(value: unknown): string {
  return JSON.stringify(value, null, 2);
}
