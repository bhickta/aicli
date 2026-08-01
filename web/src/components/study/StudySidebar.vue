<script setup lang="ts">
import type { StudyCopyRecord } from "../../types";
import StudyBatchControls from "./StudyBatchControls.vue";
import StudyCopyTable from "./StudyCopyTable.vue";

defineProps<{
  query: string;
  summary: string;
  status: string;
  copies: StudyCopyRecord[];
  activeId: string;
  selectedIds: string[];
  providerId: string;
  ocrModel: string;
  questionModel: string;
  boundaryModel: string;
  reportModel: string;
  parallelism: number;
  forceRerun: boolean;
  running: boolean;
}>();

const emit = defineEmits<{
  "update:query": [value: string];
  "update:providerId": [value: string];
  "update:ocrModel": [value: string];
  "update:questionModel": [value: string];
  "update:boundaryModel": [value: string];
  "update:reportModel": [value: string];
  "update:parallelism": [value: number];
  "update:forceRerun": [value: boolean];
  search: [];
  clear: [];
  runSelected: [];
  generateMetadata: [];
  open: [id: string];
  toggle: [id: string];
}>();
</script>

<template>
  <aside class="study-sidebar">
    <div class="study-toolbar">
      <input
        :value="query"
        type="search"
        placeholder="Search copies, topper, paper, topic"
        @input="emit('update:query', ($event.target as HTMLInputElement).value)"
        @keyup.enter="emit('search')"
      />
      <button type="button" @click="emit('search')">Search</button>
    </div>
    <p class="study-summary">{{ summary }} · {{ status }}</p>
    <StudyBatchControls
      :selected-count="selectedIds.length"
      :provider-id="providerId"
      :ocr-model="ocrModel"
      :question-model="questionModel"
      :boundary-model="boundaryModel"
      :report-model="reportModel"
      :parallelism="parallelism"
      :force-rerun="forceRerun"
      :running="running"
      @update:provider-id="emit('update:providerId', $event)"
      @update:ocr-model="emit('update:ocrModel', $event)"
      @update:question-model="emit('update:questionModel', $event)"
      @update:boundary-model="emit('update:boundaryModel', $event)"
      @update:report-model="emit('update:reportModel', $event)"
      @update:parallelism="emit('update:parallelism', $event)"
      @update:force-rerun="emit('update:forceRerun', $event)"
      @run-selected="emit('runSelected')"
      @generate-metadata="emit('generateMetadata')"
      @clear="emit('clear')"
    />
    <StudyCopyTable
      :copies="copies"
      :active-id="activeId"
      :selected-ids="selectedIds"
      @open="emit('open', $event)"
      @toggle="emit('toggle', $event)"
    />
  </aside>
</template>
