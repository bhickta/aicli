import { folderPickerStartPath, relativeToVault } from "../../features/zettel/paths";
import type { ZettelConfig, ZettelFolderField } from "../../features/zettel/types";
import { api } from "../../lib/api";
import type { useZettelRunner } from "../useZettelRunner";
import type { useZettelReports } from "./useZettelReports";

type ZettelRunner = ReturnType<typeof useZettelRunner>;
type ZettelReports = ReturnType<typeof useZettelReports>;

export function useZettelFolders(config: ZettelConfig, runner: ZettelRunner, reports: ZettelReports) {
  async function pickVault() {
    await runner.run("Opening vault picker", async () => {
      reports.clearApiUsage();
      const query = config.vaultPath ? `?path=${encodeURIComponent(config.vaultPath)}` : "";
      const picked = await api<{ path: string }>(`/api/fs/pick-directory${query}`);
      config.vaultPath = picked.path;
      runner.status.value = "Vault selected";
      runner.result.value = picked.path;
    });
  }

  async function pickZettelFolder(field: ZettelFolderField, label: string) {
    await runner.run(`Choosing ${label}`, async () => {
      reports.clearApiUsage();
      if (!config.vaultPath.trim()) throw new Error("Select a vault first");
      const startPath = folderPickerStartPath(config, field);
      const query = startPath ? `?path=${encodeURIComponent(startPath)}` : "";
      const picked = await api<{ path: string }>(`/api/fs/pick-directory${query}`);
      const relative = relativeToVault(config, picked.path);
      if (!relative) throw new Error(`${label} must be inside the selected vault`);
      config[field] = relative;
      runner.status.value = `${label} selected`;
      runner.result.value = `${label}: ${relative}`;
    });
  }

  return {
    pickVault,
    pickZettelFolder,
  };
}
