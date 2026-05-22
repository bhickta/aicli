<script setup lang="ts">
import { computed } from "vue";
import type { InboxMergeReport } from "../../../types";
import type { ZettelFolderField } from "../../../features/zettel/types";
import ZettelFolderChooser from "../common/ZettelFolderChooser.vue";
import ZettelInboxReport from "./ZettelInboxReport.vue";
import ZettelRunSizeControl from "./ZettelRunSizeControl.vue";
import ZettelSection from "../common/ZettelSection.vue";

const props = defineProps<{
  inboxFolder: string;
  rootFolder: string;
  inboxLimit: number;
  busy: boolean;
  canRun: boolean;
  canUseFolders: boolean;
  report: InboxMergeReport | null;
}>();

const emit = defineEmits<{
  run: [];
  buildIndex: [];
  pickFolder: [field: ZettelFolderField, label: string];
  updateInboxLimit: [value: number];
}>();

const limitModel = computed({
  get: () => props.inboxLimit,
  set: (value: number) => emit("updateInboxLimit", value),
});
</script>

<template>
  <ZettelSection title="Inbox merge" description="Embed source notes, find semantic matches, then let one AI merge call write final atomic notes.">
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

    <ZettelRunSizeControl v-model="limitModel" :disabled="busy" />

    <div class="zettel-inline-actions">
      <button type="button" :disabled="busy" @click="emit('buildIndex')">Build Index</button>
    </div>

    <ZettelInboxReport :report="report" />
    <p v-if="!report" class="muted">Run report, changed files, pending notes, and rollback id appear here.</p>
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

.mod-cta {
  border-color: #6ea8fe;
  background: #2d405e;
}
</style>
