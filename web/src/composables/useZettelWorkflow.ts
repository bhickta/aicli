import { computed, onMounted, reactive, shallowRef, watch } from "vue";
import { api } from "../lib/api";
import { stringify } from "../lib/format";
import type { ApiCallUsage, InboxMergeReport } from "../types";
import {
  candidateLimitOptions,
  createZettelConfig,
  embeddingBatchSizeOptions,
  embeddingWorkerOptions,
  inboxWorkerOptions,
  persistZettelConfig,
  persistZettelMode,
  promptOptions,
  readZettelMode,
} from "../features/zettel/config";
import { buildZettelPayload } from "../features/zettel/payload";
import { folderPickerStartPath, relativeToVault } from "../features/zettel/paths";
import type {
  ZettelConfig,
  ZettelFolderField,
  ZettelMode,
  ZettelProviderSettingsPatch,
} from "../features/zettel/types";
import { useZettelRunner } from "./useZettelRunner";

export function useZettelWorkflow() {
  const config = reactive<ZettelConfig>(createZettelConfig());

  const inboxReport = shallowRef<InboxMergeReport | null>(null);
  const apiUsage = shallowRef<ApiCallUsage | null>(null);
  const rollbackJobID = shallowRef("");
  const mode = shallowRef<ZettelMode>(readZettelMode());
  const runner = useZettelRunner();

  const canRunInboxMerge = computed(() => Boolean(config.vaultPath.trim()) && !runner.busy.value);
  const canUseVaultFolders = computed(() => Boolean(config.vaultPath.trim()) && !runner.busy.value);
  onMounted(() => {
    if (!config.vaultPath) return;
    runner.status.value = "Vault ready";
  });

  watch(config, () => {
    persistZettelConfig(config);
  });

  watch(mode, () => {
    persistZettelMode(mode.value);
  });

  function updateProviderSettings(value: ZettelProviderSettingsPatch) {
    Object.assign(config, value);
  }

  function updateConfig(field: keyof ZettelConfig, value: ZettelConfig[keyof ZettelConfig]) {
    Object.assign(config, { [field]: value });
  }

  function basePayload() {
    return buildZettelPayload(config);
  }

  async function pickVault() {
    await runner.run("Opening vault picker", async () => {
      apiUsage.value = null;
      const query = config.vaultPath ? `?path=${encodeURIComponent(config.vaultPath)}` : "";
      const picked = await api<{ path: string }>(`/api/fs/pick-directory${query}`);
      config.vaultPath = picked.path;
      runner.status.value = "Vault selected";
      runner.result.value = picked.path;
    });
  }

  async function pickZettelFolder(field: ZettelFolderField, label: string) {
    await runner.run(`Choosing ${label}`, async () => {
      apiUsage.value = null;
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

  async function buildIndex() {
    apiUsage.value = null;
    await runner.runWorkflow("Building zettel index", "/api/workflows/zettel/index", basePayload(), (output) => {
      updateApiUsage(output);
      runner.status.value = "Embedding index is ready";
      runner.result.value = stringify(output);
    });
  }

  async function rollback() {
    apiUsage.value = null;
    await runner.runWorkflow("Rolling back zettel merge", "/api/workflows/zettel/rollback", {
      ...basePayload(),
      job_id: rollbackJobID.value,
    }, (output) => {
      updateApiUsage(output);
      runner.status.value = "Rollback completed";
      runner.result.value = stringify(output);
    });
  }

  async function runInboxMerge() {
    apiUsage.value = null;
    await runner.runWorkflow("Running inbox merge", "/api/workflows/zettel/inbox-merge", basePayload(), (output) => {
      updateApiUsage(output);
      const response = output as InboxMergeReport;
      inboxReport.value = response;
      rollbackJobID.value = response.run_id || "";
      runner.status.value = `Inbox merge completed: ${response.processed_count} processed, ${response.pending_count} pending, ${response.failed_count} failed`;
      runner.result.value = stringify(output);
    });
  }

  function updateApiUsage(output: unknown) {
    apiUsage.value = extractApiUsage(output);
  }

  function extractApiUsage(output: unknown): ApiCallUsage | null {
    if (!output || typeof output !== "object") return null;
    const root = output as { api_calls?: ApiCallUsage; proposal?: { api_calls?: ApiCallUsage } };
    return root.api_calls || root.proposal?.api_calls || null;
  }

  return {
    config,
    inboxReport,
    apiUsage,
    status: runner.status,
    result: runner.result,
    busy: runner.busy,
    mode,
    candidateLimitOptions,
    promptOptions,
    embeddingBatchSizeOptions,
    embeddingWorkerOptions,
    inboxWorkerOptions,
    canRunInboxMerge,
    canUseVaultFolders,
    rawResultSummary: runner.rawResultSummary,
    progressClass: runner.progressClass,
    progressStyle: runner.progressStyle,
    updateProviderSettings,
    updateConfig,
    pickVault,
    pickZettelFolder,
    buildIndex,
    rollback,
    runInboxMerge,
  };
}
