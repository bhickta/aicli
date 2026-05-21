import { computed, onMounted, reactive, shallowRef, watch } from "vue";
import { api } from "../lib/api";
import { stringify } from "../lib/format";
import type { ApiCallUsage, InboxMergeReport } from "../types";
import {
  candidateLimitOptions,
  createZettelConfig,
  persistZettelConfig,
  persistZettelMode,
  promptOptions,
  readZettelMode,
  thresholdOptions,
  validationThresholdOptions,
} from "../features/zettel/config";
import { buildZettelPayload } from "../features/zettel/payload";
import { folderPickerStartPath, relativeToVault } from "../features/zettel/paths";
import type {
  ZettelCandidate,
  ZettelConfig,
  ZettelFolderField,
  ZettelMode,
  ZettelProposal,
  ZettelProviderSettingsPatch,
} from "../features/zettel/types";
import { useZettelRunner } from "./useZettelRunner";

export function useZettelWorkflow() {
  const config = reactive<ZettelConfig>(createZettelConfig());

  const candidates = shallowRef<ZettelCandidate[]>([]);
  const notes = shallowRef<string[]>([]);
  const selectedPaths = shallowRef<string[]>([]);
  const proposal = shallowRef<ZettelProposal | null>(null);
  const inboxReport = shallowRef<InboxMergeReport | null>(null);
  const apiUsage = shallowRef<ApiCallUsage | null>(null);
  const notesStatus = shallowRef("Load notes after selecting a vault.");
  const rollbackJobID = shallowRef("");
  const mode = shallowRef<ZettelMode>(readZettelMode());
  const runner = useZettelRunner();

  const selectedCandidates = computed(() => {
    const selected = new Set(selectedPaths.value);
    return candidates.value.filter((candidate) => selected.has(candidate.path));
  });
  const canSuggest = computed(() => Boolean(config.vaultPath.trim() && config.activePath.trim()) && !runner.busy.value);
  const canPreview = computed(() => selectedCandidates.value.length > 0 && !runner.busy.value);
  const canApply = computed(() => Boolean(proposal.value) && !runner.busy.value);
  const canRunInboxMerge = computed(() => Boolean(config.vaultPath.trim()) && !runner.busy.value);
  const canUseVaultFolders = computed(() => Boolean(config.vaultPath.trim()) && !runner.busy.value);
  const proposalQuality = computed(() => {
    if (!proposal.value) return "";
    const coverage = formatScore(proposal.value.coverage?.score);
    const judge = formatScore(proposal.value.judge?.score);
    return `coverage ${coverage} | judge ${judge} | ${proposal.value.judge?.verdict || "unknown"}`;
  });
  onMounted(() => {
    if (!config.vaultPath) return;
    void refreshNotes().catch((error) => {
      notesStatus.value = error instanceof Error ? error.message : "Unable to load notes";
    });
  });

  watch(config, () => {
    persistZettelConfig(config);
  });

  watch(mode, () => {
    persistZettelMode(mode.value);
  });

  function updateProviderSettings(value: ZettelProviderSettingsPatch) {
    Object.assign(config, value);
    config.providerId = config.candidateProviderId;
    config.judgeModel = config.candidateModel;
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
      await refreshNotes();
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

  async function loadNotes() {
    await runner.run("Loading notes", refreshNotes);
  }

  async function refreshNotes() {
    apiUsage.value = null;
    notesStatus.value = "Loading notes...";
    const output = await api<{ notes?: string[]; count?: number }>("/api/workflows/zettel/notes", {
      method: "POST",
      body: JSON.stringify(basePayload()),
    });
    notes.value = output.notes || [];
    notesStatus.value = `${output.count ?? notes.value.length} note(s) loaded`;
    if (config.activePath && !notes.value.includes(config.activePath)) {
      notesStatus.value += " | active note is outside the loaded list";
    }
  }

  async function buildIndex() {
    apiUsage.value = null;
    await runner.runWorkflow("Building zettel index", "/api/workflows/zettel/index", basePayload(), (output) => {
      updateApiUsage(output);
      runner.status.value = "Embedding index is ready";
      runner.result.value = stringify(output);
    });
  }

  async function suggest() {
    apiUsage.value = null;
    await runner.runWorkflow("Finding merge candidates", "/api/workflows/zettel/suggest", {
      ...basePayload(),
      active_path: config.activePath,
    }, (output) => {
      updateApiUsage(output);
      const response = output as { candidates?: ZettelCandidate[] };
      candidates.value = response.candidates || [];
      selectedPaths.value = [];
      proposal.value = null;
      runner.status.value = candidates.value.length ? `${candidates.value.length} mergeable candidate(s) found` : "No mergeable candidates found";
      runner.result.value = stringify(output);
    });
  }

  async function previewMerge() {
    apiUsage.value = null;
    const selections = selectedCandidates.value.map((candidate) => ({
      path: candidate.path,
      source_line_ranges: candidate.source_line_ranges,
    }));
    await runner.runWorkflow("Preparing merge preview", "/api/workflows/zettel/propose", {
      ...basePayload(),
      active_path: config.activePath,
      selections,
    }, (output) => {
      updateApiUsage(output);
      const response = output as { proposal?: ZettelProposal };
      proposal.value = response.proposal || null;
      runner.status.value = proposal.value ? "Merge preview ready" : "Merge preview returned no proposal";
      runner.result.value = stringify(output);
    });
  }

  async function applyMerge() {
    if (!proposal.value) return;
    apiUsage.value = null;
    await runner.runWorkflow("Applying approved merge", "/api/workflows/zettel/apply", {
      ...basePayload(),
      proposal: proposal.value,
    }, (output) => {
      updateApiUsage(output);
      const response = output as { job_id?: string };
      rollbackJobID.value = response.job_id || proposal.value?.id || "";
      runner.status.value = `Applied merge ${rollbackJobID.value}`;
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

  function toggleCandidate(path: string, checked: boolean) {
    const next = new Set(selectedPaths.value);
    if (checked) next.add(path);
    else next.delete(path);
    selectedPaths.value = Array.from(next);
    proposal.value = null;
  }

  function formatScore(value: unknown) {
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) return "n/a";
    return parsed.toFixed(2);
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
    candidates,
    notes,
    selectedPaths,
    proposal,
    inboxReport,
    apiUsage,
    status: runner.status,
    result: runner.result,
    notesStatus,
    busy: runner.busy,
    mode,
    candidateLimitOptions,
    thresholdOptions,
    validationThresholdOptions,
    promptOptions,
    canSuggest,
    canPreview,
    canApply,
    canRunInboxMerge,
    canUseVaultFolders,
    rawResultSummary: runner.rawResultSummary,
    proposalQuality,
    progressClass: runner.progressClass,
    progressStyle: runner.progressStyle,
    updateProviderSettings,
    updateConfig,
    pickVault,
    pickZettelFolder,
    loadNotes,
    buildIndex,
    suggest,
    previewMerge,
    applyMerge,
    rollback,
    runInboxMerge,
    toggleCandidate,
  };
}
