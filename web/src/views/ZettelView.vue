<script setup lang="ts">
import ZettelWorkflowTabs from "../components/zettel/common/ZettelWorkflowTabs.vue";
import ZettelInboxPanel from "../components/zettel/inbox/ZettelInboxPanel.vue";
import ZettelManualPanel from "../components/zettel/manual/ZettelManualPanel.vue";
import ZettelSettingsPanel from "../components/zettel/settings/ZettelSettingsPanel.vue";
import ZettelStatusPanel from "../components/zettel/shell/ZettelStatusPanel.vue";
import ZettelVaultPanel from "../components/zettel/shell/ZettelVaultPanel.vue";
import { useZettelWorkflow } from "../composables/useZettelWorkflow";

const {
  config,
  candidates,
  notes,
  selectedPaths,
  proposal,
  inboxReport,
  apiUsage,
  status,
  result,
  notesStatus,
  busy,
  mode,
  candidateLimitOptions,
  thresholdOptions,
  validationThresholdOptions,
  promptOptions,
  embeddingBatchSizeOptions,
  embeddingWorkerOptions,
  canSuggest,
  canPreview,
  canApply,
  canRunInboxMerge,
  canUseVaultFolders,
  rawResultSummary,
  proposalQuality,
  progressClass,
  progressStyle,
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
} = useZettelWorkflow();
</script>

<template>
  <div class="panel grid zettel-panel">
    <div class="zettel-title-row">
      <div>
        <h2>Zettelkasten</h2>
        <p class="muted">Inbox merge by default. Manual review and provider setup are separate.</p>
      </div>
      <ZettelWorkflowTabs v-model="mode" />
    </div>

    <ZettelVaultPanel :vault-path="config.vaultPath" :busy="busy" @pick="pickVault" />

    <ZettelInboxPanel
      v-if="mode === 'inbox'"
      :inbox-folder="config.inboxFolder"
      :root-folder="config.rootFolder"
      :inbox-limit="config.inboxLimit"
      :busy="busy"
      :can-run="canRunInboxMerge"
      :can-use-folders="canUseVaultFolders"
      :report="inboxReport"
      @run="runInboxMerge"
      @build-index="buildIndex"
      @pick-folder="pickZettelFolder"
      @update-inbox-limit="updateConfig('inboxLimit', $event)"
    />

    <ZettelManualPanel
      v-if="mode === 'manual'"
      :active-path="config.activePath"
      :notes="notes"
      :notes-status="notesStatus"
      :busy="busy"
      :candidates="candidates"
      :selected-paths="selectedPaths"
      :proposal="proposal"
      :proposal-quality="proposalQuality"
      :can-suggest="canSuggest"
      :can-preview="canPreview"
      :can-apply="canApply"
      @update-active-path="updateConfig('activePath', $event)"
      @load-notes="loadNotes"
      @suggest="suggest"
      @preview="previewMerge"
      @apply="applyMerge"
      @toggle-candidate="toggleCandidate"
    />

    <ZettelSettingsPanel
      v-if="mode === 'settings'"
      :config="config"
      :can-use-vault-folders="canUseVaultFolders"
      :candidate-limit-options="candidateLimitOptions"
      :threshold-options="thresholdOptions"
      :validation-threshold-options="validationThresholdOptions"
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
