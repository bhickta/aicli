<script setup lang="ts">
import { copyMetadataChips } from "../../lib/studyMetadata";
import type { StudyCopyRecord } from "../../types";
import StudyMetadataChips from "./StudyMetadataChips.vue";

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
  const parts = [copy.render_status, copy.ocr_status, copy.question_status, copy.report_status]
    .filter(Boolean);
  if (parts.length > 0) {
    return parts.join(" / ");
  }
  return copy.status || "pending";
}

function statusClass(copy: StudyCopyRecord) {
  const status = copy.status || "pending";
  if (status === "failed" || copy.last_error) return "status-failed";
  if (status === "completed" || status === "ready" || (copy.ocr_status === "ready" && copy.question_status === "ready")) return "status-ready";
  if (status === "running" || status === "processing" || copy.render_status === "running" || copy.ocr_status === "running") return "status-running";
  return "status-pending";
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
        <StudyMetadataChips :chips="copyMetadataChips(copy).slice(0, 3)" />
      </span>
      <span>{{ copy.page_count || "-" }}</span>
      <span>{{ copy.question_count || "-" }}</span>
      <span class="study-status">
        <span class="study-status-badge" :class="statusClass(copy)">
          <svg v-if="statusClass(copy) === 'status-running'" class="spinner-icon" xmlns="http://www.w3.org/2000/svg" width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round"><line x1="12" y1="2" x2="12" y2="6"></line><line x1="12" y1="18" x2="12" y2="22"></line><line x1="4.93" y1="4.93" x2="7.76" y2="7.76"></line><line x1="16.24" y1="16.24" x2="19.07" y2="19.07"></line><line x1="2" y1="12" x2="6" y2="12"></line><line x1="18" y1="12" x2="22" y2="12"></line><line x1="6.83" y1="17.17" x2="9.66" y2="14.34"></line><line x1="14.34" y1="9.66" x2="17.17" y2="6.83"></line></svg>
          {{ statusLabel(copy) }}
        </span>
      </span>
    </button>
    <div v-if="!copies.length" class="study-empty">No tracked study copies yet.</div>
  </div>
</template>
