<script setup lang="ts">
import { computed } from "vue";
import type { MetadataReport } from "../../../types";
import type { ZettelFolderField } from "../../../features/zettel/types";
import WorkerCountControl from "../common/WorkerCountControl.vue";
import ZettelFolderChooser from "../common/ZettelFolderChooser.vue";
import ZettelRunSizeControl from "../inbox/ZettelRunSizeControl.vue";
import ZettelSection from "../common/ZettelSection.vue";
import ZettelMetadataReport from "./ZettelMetadataReport.vue";

const props = defineProps<{
  metadataFolder: string;
  metadataLimit: number;
  metadataWorkers: number;
  metadataOverwrite: boolean;
  metadataWorkerOptions: number[];
  busy: boolean;
  canRun: boolean;
  canUseFolders: boolean;
  report: MetadataReport | null;
}>();

const emit = defineEmits<{
  run: [];
  pickFolder: [field: ZettelFolderField, label: string];
  updateMetadataLimit: [value: number];
  updateMetadataWorkers: [value: number];
  updateMetadataOverwrite: [value: boolean];
}>();

const limitModel = computed({
  get: () => props.metadataLimit,
  set: (value: number) => emit("updateMetadataLimit", value),
});

const workersModel = computed({
  get: () => props.metadataWorkers,
  set: (value: number) => emit("updateMetadataWorkers", value),
});

const overwriteModel = computed({
  get: () => props.metadataOverwrite,
  set: (value: boolean) => emit("updateMetadataOverwrite", value),
});
</script>

<template>
  <ZettelSection title="Metadata generator" description="Add detailed title, keyword summary, and recall questions to Markdown notes.">
    <template #actions>
      <button type="button" class="mod-cta" :disabled="!canRun" @click="emit('run')">Run Metadata</button>
    </template>

    <div class="zettel-folder-grid">
      <ZettelFolderChooser
        label="Notes folder"
        :value="metadataFolder"
        description="Folder whose Markdown notes receive metadata"
        :disabled="!canUseFolders"
        @choose="emit('pickFolder', 'metadataFolder', 'metadata notes folder')"
      />
    </div>

    <div class="metadata-run-grid">
      <ZettelRunSizeControl
        v-model="limitModel"
        :disabled="busy"
        label="Run size"
        aria-label="Metadata run size"
        empty-text="All notes"
        active-text="First"
      />
      <WorkerCountControl
        id="zettel-metadata-workers"
        v-model="workersModel"
        label="Parallel calls"
        :options="metadataWorkerOptions"
        :disabled="busy"
        helper="Each note uses one metadata chat call."
      />
      <label class="zettel-checkbox">
        <input v-model="overwriteModel" type="checkbox" :disabled="busy">
        Overwrite existing metadata
      </label>
    </div>

    <ZettelMetadataReport :report="report" />
    <p v-if="!report" class="muted">Metadata reports appear here after a run.</p>
  </ZettelSection>
</template>

<style scoped>
.zettel-folder-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}

.metadata-run-grid {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(180px, 240px) minmax(180px, auto);
  gap: 12px;
  align-items: end;
}

.zettel-checkbox {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 38px;
}

.mod-cta {
  border-color: #6ea8fe;
  background: #2d405e;
}

@media (max-width: 760px) {
  .metadata-run-grid {
    grid-template-columns: 1fr;
  }
}
</style>
