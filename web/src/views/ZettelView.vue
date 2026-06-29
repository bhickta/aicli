<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "../components/layout/PageHeader.vue";
import PageTabs from "../components/layout/PageTabs.vue";
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
const route = useRoute();
const router = useRouter();
const tabs = computed(() => [
  { label: "Inbox", to: { name: "zettel", params: { mode: "inbox" } }, active: mode.value === "inbox" },
  { label: "Metadata", to: { name: "zettel", params: { mode: "metadata" } }, active: mode.value === "metadata" },
  { label: "Training", to: { name: "zettel", params: { mode: "training" } }, active: mode.value === "training" },
  { label: "Settings", to: { name: "zettel", params: { mode: "settings" } }, active: mode.value === "settings" },
]);

watch(() => route.params.mode, (nextMode) => {
  if (nextMode === "metadata" || nextMode === "training" || nextMode === "settings" || nextMode === "inbox") {
    mode.value = nextMode;
    return;
  }
  void router.replace({ name: "zettel", params: { mode: mode.value || "inbox" } });
}, { immediate: true });
</script>

<template>
  <div class="panel grid zettel-panel">
    <PageHeader title="Zettelkasten" description="Embed, find semantic matches, and merge notes into the vault.">
      <template #actions>
        <PageTabs :tabs="tabs" label="Zettelkasten workflow" />
      </template>
    </PageHeader>

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

</style>
