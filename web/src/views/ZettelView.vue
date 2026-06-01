<script setup lang="ts">
import ZettelWorkflowTabs from "../components/zettel/common/ZettelWorkflowTabs.vue";
import ZettelInboxPanel from "../components/zettel/inbox/ZettelInboxPanel.vue";
import ZettelMetadataPanel from "../components/zettel/metadata/ZettelMetadataPanel.vue";
import ZettelSettingsPanel from "../components/zettel/settings/ZettelSettingsPanel.vue";
import ZettelStatusPanel from "../components/zettel/shell/ZettelStatusPanel.vue";
import ZettelVaultPanel from "../components/zettel/shell/ZettelVaultPanel.vue";
import ZettelTrainingPanel from "../components/zettel/training/ZettelTrainingPanel.vue";
import { useZettelWorkflow } from "../composables/useZettelWorkflow";

const {
  config,
  inboxReport,
  metadataReport,
  trainingReport,
  candidatePreview,
  apiUsage,
  status,
  result,
  busy,
  mode,
  candidateLimitOptions,
  promptOptions,
  embeddingBatchSizeOptions,
  embeddingWorkerOptions,
  inboxWorkerOptions,
  metadataWorkerOptions,
  canRunInboxMerge,
  canRunMetadata,
  canExportTraining,
  canUseVaultFolders,
  rawResultSummary,
  progressClass,
  progressStyle,
  updateProviderSettings,
  updateConfig,
  pickVault,
  pickZettelFolder,
  buildIndex,
  previewInboxCandidates,
  rollback,
  runInboxMerge,
  runMetadata,
  exportTrainingData,
} = useZettelWorkflow();
</script>

<template>
  <div class="panel grid zettel-panel">
    <div class="zettel-title-row">
      <div>
        <h2>Zettelkasten</h2>
        <p class="muted">Simple inbox flow: embed, find semantic matches, then ask AI for final merged notes.</p>
      </div>
      <ZettelWorkflowTabs v-model="mode" />
    </div>

    <ZettelVaultPanel :vault-path="config.vaultPath" :busy="busy" @pick="pickVault" />

    <ZettelInboxPanel
      v-if="mode === 'inbox'"
      :inbox-folder="config.inboxFolder"
      :root-folder="config.rootFolder"
      :inbox-limit="config.inboxLimit"
      :inbox-workers="config.inboxWorkers"
      :inbox-random="config.inboxRandom"
      :inbox-worker-options="inboxWorkerOptions"
      :busy="busy"
      :can-run="canRunInboxMerge"
      :can-use-folders="canUseVaultFolders"
      :candidate-preview="candidatePreview"
      :report="inboxReport"
      @run="runInboxMerge"
      @preview-candidates="previewInboxCandidates"
      @build-index="buildIndex"
      @pick-folder="pickZettelFolder"
      @update-inbox-limit="updateConfig('inboxLimit', $event)"
      @update-inbox-workers="updateConfig('inboxWorkers', $event)"
      @update-inbox-random="updateConfig('inboxRandom', $event)"
    />

    <ZettelMetadataPanel
      v-if="mode === 'metadata'"
      :metadata-folder="config.metadataFolder"
      :metadata-limit="config.metadataLimit"
      :metadata-workers="config.metadataWorkers"
      :metadata-overwrite="config.metadataOverwrite"
      :metadata-worker-options="metadataWorkerOptions"
      :busy="busy"
      :can-run="canRunMetadata"
      :can-use-folders="canUseVaultFolders"
      :report="metadataReport"
      @run="runMetadata"
      @pick-folder="pickZettelFolder"
      @update-metadata-limit="updateConfig('metadataLimit', $event)"
      @update-metadata-workers="updateConfig('metadataWorkers', $event)"
      @update-metadata-overwrite="updateConfig('metadataOverwrite', $event)"
    />

    <ZettelTrainingPanel
      v-if="mode === 'training'"
      :busy="busy"
      :can-run="canExportTraining"
      :strict="config.trainingStrict"
      :report="trainingReport"
      @run="exportTrainingData"
      @update-strict="updateConfig('trainingStrict', $event)"
    />

    <ZettelSettingsPanel
      v-if="mode === 'settings'"
      :config="config"
      :can-use-vault-folders="canUseVaultFolders"
      :candidate-limit-options="candidateLimitOptions"
      :prompt-options="promptOptions"
      :embedding-batch-size-options="embeddingBatchSizeOptions"
      :embedding-worker-options="embeddingWorkerOptions"
      @update-provider-settings="updateProviderSettings"
      @pick-folder="pickZettelFolder"
      @update-config="updateConfig"
    />

    <ZettelStatusPanel
      :status="status"
      :busy="busy"
      :progress-class="progressClass"
      :progress-style="progressStyle"
      :api-usage="apiUsage"
      :raw-result-summary="rawResultSummary"
      :result="result"
      @rollback="rollback"
    />
  </div>
</template>

<style scoped>
.zettel-panel {
  max-width: 1280px;
}

.zettel-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.zettel-title-row p {
  margin: 6px 0 0;
}

@media (max-width: 760px) {
  .zettel-title-row {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
