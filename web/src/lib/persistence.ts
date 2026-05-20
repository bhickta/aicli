export function readStoredString(key: string, fallback = "") {
  try {
    return globalThis.localStorage?.getItem(key) ?? fallback;
  } catch {
    return fallback;
  }
}

export function writeStoredString(key: string, value: string) {
  try {
    globalThis.localStorage?.setItem(key, value);
  } catch {
    // Local storage can be unavailable in private or restricted browser contexts.
  }
}

export function readStoredRecord(key: string): Record<string, unknown> {
  try {
    const raw = globalThis.localStorage?.getItem(key);
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    return parsed as Record<string, unknown>;
  } catch {
    return {};
  }
}

export function writeStoredJSON(key: string, value: unknown) {
  try {
    globalThis.localStorage?.setItem(key, JSON.stringify(value));
  } catch {
    // Local storage can be unavailable in private or restricted browser contexts.
  }
}
