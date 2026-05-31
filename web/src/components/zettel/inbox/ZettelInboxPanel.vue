<script setup lang="ts">
import { computed } from "vue";
import type { InboxCandidatePreviewReport, InboxMergeReport } from "../../../types";
import type { ZettelFolderField } from "../../../features/zettel/types";
import WorkerCountControl from "../common/WorkerCountControl.vue";
import ZettelFolderChooser from "../common/ZettelFolderChooser.vue";
import ZettelCandidatePreview from "./ZettelCandidatePreview.vue";
import ZettelInboxReport from "./ZettelInboxReport.vue";
import ZettelRunSizeControl from "./ZettelRunSizeControl.vue";
import ZettelSection from "../common/ZettelSection.vue";

const props = defineProps<{
  inboxFolder: string;
  rootFolder: string;
  inboxLimit: number;
  inboxWorkers: number;
  inboxRandom: boolean;
  inboxWorkerOptions: number[];
  busy: boolean;
  canRun: boolean;
  canUseFolders: boolean;
  candidatePreview: InboxCandidatePreviewReport | null;
  report: InboxMergeReport | null;
}>();

const emit = defineEmits<{
  run: [];
  previewCandidates: [];
  buildIndex: [];
  pickFolder: [field: ZettelFolderField, label: string];
  updateInboxLimit: [value: number];
  updateInboxWorkers: [value: number];
  updateInboxRandom: [value: boolean];
}>();

const limitModel = computed({
  get: () => props.inboxLimit,
  set: (value: number) => emit("updateInboxLimit", value),
});

const workersModel = computed({
  get: () => props.inboxWorkers,
  set: (value: number) => emit("updateInboxWorkers", value),
});

const randomModel = computed({
  get: () => props.inboxRandom,
  set: (value: boolean) => emit("updateInboxRandom", value),
});
</script>

<template>
  <ZettelSection title="Inbox merge" description="Embed source notes, find semantic matches, and ask AI for final merged notes.">
    <template #actions>
      <button type="button" class="mod-cta" :disabled="!canRun" @click="emit('run')">Run Inbox Merge</button>
    </template>

    <div class="zettel-folder-grid">
      <ZettelFolderChooser
        label="Source inbox"
        :value="inboxFolder"
        description="New atomic notes waiting to be merged"
        :disabled="!canUseFolders"
        @choose="emit('pickFolder', 'inboxFolder', 'source inbox')"
      />
      <ZettelFolderChooser
        label="Destination notes"
        :value="rootFolder"
        description="Existing zettelkasten tree that receives final merged notes"
        :disabled="!canUseFolders"
        @choose="emit('pickFolder', 'rootFolder', 'destination notes')"
      />
    </div>

    <div class="zettel-run-grid">
      <ZettelRunSizeControl v-model="limitModel" :disabled="busy" />
      <WorkerCountControl
        id="zettel-inbox-workers"
        v-model="workersModel"
        label="Parallel calls"
        :options="inboxWorkerOptions"
        :disabled="busy"
        helper="AI calls run in parallel; file writes stay serialized."
      />
      <label class="zettel-checkbox">
        <input v-model="randomModel" type="checkbox" :disabled="busy">
        Random notes
      </label>
    </div>

    <div class="zettel-inline-actions">
      <button type="button" :disabled="busy || !canRun" @click="emit('previewCandidates')">Preview Embedding Matches</button>
      <button type="button" :disabled="busy" @click="emit('buildIndex')">Build Index</button>
    </div>

    <ZettelCandidatePreview :report="candidatePreview" />
    <ZettelInboxReport :report="report" />
    <p v-if="!candidatePreview && !report" class="muted">Previewed candidates and merge reports appear here.</p>
  </ZettelSection>
</template>

<style scoped>
.zettel-folder-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 12px;
}

.zettel-inline-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.zettel-inline-actions button {
  text-align: center;
}

.zettel-run-grid {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) minmax(180px, 240px) minmax(140px, auto);
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
  .zettel-run-grid {
    grid-template-columns: 1fr;
  }
}
</style>
