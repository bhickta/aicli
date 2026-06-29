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
  parallelism: number;
  forceRerun: boolean;
  running: boolean;
}>();

const emit = defineEmits<{
  "update:query": [value: string];
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
      :parallelism="parallelism"
      :force-rerun="forceRerun"
      :running="running"
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
