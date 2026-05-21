import type { ZettelConfig, ZettelFolderField } from "./types";

export function folderPickerStartPath(config: ZettelConfig, field: ZettelFolderField) {
  const vaultPath = normalizePath(config.vaultPath);
  const current = normalizePath(String(config[field] || ""));
  if (!vaultPath) return "";
  if (!current) return vaultPath;
  if (current.startsWith("/")) return current;
  return `${vaultPath}/${current.replace(/^\/+/, "")}`;
}

export function relativeToVault(config: ZettelConfig, path: string) {
  const vaultPath = normalizePath(config.vaultPath);
  const pickedPath = normalizePath(path);
  if (!vaultPath || !pickedPath || pickedPath === vaultPath) return "";
  if (!pickedPath.startsWith(`${vaultPath}/`)) return "";
  return pickedPath.slice(vaultPath.length + 1);
}

function normalizePath(value: string) {
  return value.trim().replace(/\\/g, "/").replace(/\/+$/, "");
}
