export function uploadURLFromPath(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/");
  if (!normalized) return "";
  const marker = "/uploads/";
  const markerIndex = normalized.indexOf(marker);
  if (markerIndex >= 0) {
    return "/uploads/" + encodePathParts(normalized.slice(markerIndex + marker.length));
  }
  if (normalized.startsWith("uploads/")) {
    return "/uploads/" + encodePathParts(normalized.slice("uploads/".length));
  }
  return "";
}

function encodePathParts(path: string): string {
  return path
    .split("/")
    .filter(Boolean)
    .map((part) => encodeURIComponent(part))
    .join("/");
}
