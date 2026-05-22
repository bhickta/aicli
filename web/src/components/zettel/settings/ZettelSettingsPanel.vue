<script setup lang="ts">
import type { SelectOption, ZettelConfig, ZettelFolderField, ZettelProviderSettingsPatch } from "../../../features/zettel/types";
import ZettelFolderChooser from "../common/ZettelFolderChooser.vue";
import ZettelProviderSettings from "./ZettelProviderSettings.vue";
import ZettelSection from "../common/ZettelSection.vue";

defineProps<{
  config: ZettelConfig;
  canUseVaultFolders: boolean;
  candidateLimitOptions: number[];
  thresholdOptions: SelectOption<number>[];
  validationThresholdOptions: SelectOption<number>[];
  promptOptions: SelectOption<string>[];
  embeddingBatchSizeOptions: number[];
  embeddingWorkerOptions: number[];
}>();

const emit = defineEmits<{
  updateProviderSettings: [value: ZettelProviderSettingsPatch];
  pickFolder: [field: ZettelFolderField, label: string];
  updateConfig: [field: keyof ZettelConfig, value: ZettelConfig[keyof ZettelConfig]];
}>();
</script>

<template>
  <ZettelSection title="Settings" description="All choices are saved locally and reused on the next run.">
    <div class="zettel-settings-grid">
      <ZettelProviderSettings :settings="config" @update="emit('updateProviderSettings', $event)" />
      <div class="grid">
        <ZettelFolderChooser
          label="Index and audit data"
          :value="config.dataFolder"
          description="Cache, embeddings, reports, and rollback snapshots"
          :disabled="!canUseVaultFolders"
          @choose="emit('pickFolder', 'dataFolder', 'index and audit data')"
        />
        <div class="field-row">
          <div class="field">
            <label for="zettel-limit">Candidate limit</label>
            <select
              id="zettel-limit"
              :value="config.candidateLimit"
              @change="emit('updateConfig', 'candidateLimit', Number(($event.target as HTMLSelectElement).value))"
            >
              <option v-for="value in candidateLimitOptions" :key="value" :value="value">{{ value }} notes</option>
            </select>
          </div>
          <div class="field">
            <label for="zettel-review">Review threshold</label>
            <select
              id="zettel-review"
              :value="config.reviewThreshold"
              @change="emit('updateConfig', 'reviewThreshold', Number(($event.target as HTMLSelectElement).value))"
            >
              <option v-for="option in thresholdOptions" :key="option.value" :value="option.value">
                {{ option.label }} ({{ option.value }})
              </option>
            </select>
          </div>
          <div class="field">
            <label for="zettel-validation">Validation threshold</label>
            <select
              id="zettel-validation"
              :value="config.validationThreshold"
              @change="emit('updateConfig', 'validationThreshold', Number(($event.target as HTMLSelectElement).value))"
            >
              <option v-for="option in validationThresholdOptions" :key="option.value" :value="option.value">
                {{ option.label }} ({{ option.value }})
              </option>
            </select>
          </div>
          <div class="field">
            <label for="zettel-shorthand-prompt">Prompt mode</label>
            <select
              id="zettel-shorthand-prompt"
              :value="config.shorthandPromptPath"
              @change="emit('updateConfig', 'shorthandPromptPath', ($event.target as HTMLSelectElement).value)"
            >
              <option v-for="option in promptOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </div>
          <div class="field">
            <label for="zettel-embedding-batch-size">Embedding batch</label>
            <select
              id="zettel-embedding-batch-size"
              :value="config.embeddingBatchSize"
              @change="emit('updateConfig', 'embeddingBatchSize', Number(($event.target as HTMLSelectElement).value))"
            >
              <option v-for="value in embeddingBatchSizeOptions" :key="value" :value="value">{{ value }} notes</option>
            </select>
          </div>
          <div class="field">
            <label for="zettel-embedding-workers">Embedding workers</label>
            <select
              id="zettel-embedding-workers"
              :value="config.embeddingWorkers"
              @change="emit('updateConfig', 'embeddingWorkers', Number(($event.target as HTMLSelectElement).value))"
            >
              <option v-for="value in embeddingWorkerOptions" :key="value" :value="value">{{ value }} workers</option>
            </select>
          </div>
        </div>
      </div>
    </div>
  </ZettelSection>
</template>

<style scoped>
.zettel-settings-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}
</style>
