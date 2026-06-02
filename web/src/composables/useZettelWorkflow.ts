import { onMounted } from "vue";
import { useZettelRunner } from "./useZettelRunner";
import { useZettelActions } from "./zettel/useZettelActions";
import { useZettelConfigState } from "./zettel/useZettelConfigState";
import { useZettelFolders } from "./zettel/useZettelFolders";
import { useZettelReports } from "./zettel/useZettelReports";

export function useZettelWorkflow() {
  const runner = useZettelRunner();
  const reports = useZettelReports();
  const configState = useZettelConfigState(runner.busy, reports.resetReports);
  const folders = useZettelFolders(configState.config, runner, reports);
  const actions = useZettelActions(configState.config, runner, reports);

  onMounted(() => {
    if (!configState.config.vaultPath) return;
    runner.status.value = "Vault ready";
  });

  return {
    config: configState.config,
    inboxReport: reports.inboxReport,
    metadataReport: reports.metadataReport,
    trainingReport: reports.trainingReport,
    candidatePreview: reports.candidatePreview,
    apiUsage: reports.apiUsage,
    status: runner.status,
    result: runner.result,
    busy: runner.busy,
    mode: configState.mode,
    candidateLimitOptions: configState.candidateLimitOptions,
    promptOptions: configState.promptOptions,
    embeddingBatchSizeOptions: configState.embeddingBatchSizeOptions,
    embeddingWorkerOptions: configState.embeddingWorkerOptions,
    inboxWorkerOptions: configState.inboxWorkerOptions,
    metadataWorkerOptions: configState.metadataWorkerOptions,
    canRunInboxMerge: configState.canRunInboxMerge,
    canRunMetadata: configState.canRunMetadata,
    canExportTraining: configState.canExportTraining,
    canUseVaultFolders: configState.canUseVaultFolders,
    rawResultSummary: runner.rawResultSummary,
    progressClass: runner.progressClass,
    progressStyle: runner.progressStyle,
    updateProviderSettings: configState.updateProviderSettings,
    updateConfig: configState.updateConfig,
    pickVault: folders.pickVault,
    pickZettelFolder: folders.pickZettelFolder,
    buildIndex: actions.buildIndex,
    previewInboxCandidates: actions.previewInboxCandidates,
    rollback: actions.rollback,
    runInboxMerge: actions.runInboxMerge,
    runMetadata: actions.runMetadata,
    exportTrainingData: actions.exportTrainingData,
  };
}
