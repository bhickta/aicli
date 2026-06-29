<script setup lang="ts">
import type { StudyCopyRecord } from "../../types";

defineProps<{
  copies: StudyCopyRecord[];
  activeId?: string;
  selectedIds: string[];
}>();

const emit = defineEmits<{
  open: [id: string];
  toggle: [id: string];
}>();

function statusLabel(copy: StudyCopyRecord) {
  return [copy.render_status, copy.ocr_status, copy.question_status, copy.report_status]
    .filter(Boolean)
    .join(" / ");
}
</script>

<template>
  <div class="study-table" role="table" aria-label="Study copies">
    <div class="study-table-row study-table-head" role="row">
      <span></span>
      <span>Copy</span>
      <span>Pages</span>
      <span>Questions</span>
      <span>Status</span>
    </div>
    <button
      v-for="copy in copies"
      :key="copy.id"
      class="study-table-row study-table-button"
      :class="{ active: copy.id === activeId }"
      type="button"
      @click="emit('open', copy.id)"
    >
      <input
        type="checkbox"
        :checked="selectedIds.includes(copy.id)"
        aria-label="Select copy"
        @click.stop="emit('toggle', copy.id)"
      />
      <span class="study-copy-cell">
        <strong>{{ copy.pdf_name || copy.id }}</strong>
        <small>{{ copy.candidate_name || copy.source_path || "No source metadata" }}</small>
      </span>
      <span>{{ copy.page_count || "-" }}</span>
      <span>{{ copy.question_count || "-" }}</span>
      <span class="study-status">{{ statusLabel(copy) || copy.status || "pending" }}</span>
    </button>
    <div v-if="!copies.length" class="study-empty">No tracked study copies yet.</div>
  </div>
</template>
