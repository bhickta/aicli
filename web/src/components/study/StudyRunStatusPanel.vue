<script setup lang="ts">
import { computed } from "vue";
import type { StudyBatchItemRecord, StudyBatchRecord } from "../../types";
import { useToasts } from "../../composables/useToasts";

const props = defineProps<{
  batch: StudyBatchRecord | null;
  items: StudyBatchItemRecord[];
  status: string;
  running: boolean;
}>();
const toasts = useToasts();

const progressText = computed(() => {
  if (!props.batch) return props.status || "No active run";
  return `${props.batch.completed}/${props.batch.total} complete, ${props.batch.failed} failed`;
});

const progressPercent = computed(() => {
  if (!props.batch?.total) return props.running ? 4 : 0;
  const done = props.batch.completed + props.batch.failed;
  return Math.max(4, Math.min(100, Math.round((done / props.batch.total) * 100)));
});

const visible = computed(() => props.running || Boolean(props.batch || props.status));
const failedItems = computed(() => props.items.filter((item) => item.error));
const failureText = computed(() => failedItems.value.map(errorTextForItem).join("\n\n"));

function itemCost(item: StudyBatchItemRecord) {
  if (item.cache_hit) return "cached";
  if (!item.api_calls && !item.total_tokens) return "";
  return `${item.api_calls || 0} call(s), ${item.total_tokens || 0} tokens`;
}

function errorTextForItem(item: StudyBatchItemRecord) {
  return [
    `copy_id: ${item.copy_id}`,
    `batch_id: ${item.batch_id}`,
    `status: ${item.status}`,
    item.error_kind ? `error_kind: ${item.error_kind}` : "",
    `error: ${item.error || "No error text recorded"}`,
  ].filter(Boolean).join("\n");
}

async function copyText(text: string) {
  if (!text.trim()) return;
  await navigator.clipboard.writeText(text);
  toasts.success("Error copied", "Paste it here and I can inspect it.");
}
</script>

<template>
  <section v-if="visible" class="study-run-status" :class="{ running }" aria-label="Analysis run status">
    <header>
      <div>
        <span>{{ batch?.id || "Current run" }}</span>
        <strong>{{ batch?.status || status }}</strong>
      </div>
      <span>{{ progressText }}</span>
    </header>
    <div class="study-run-progress" aria-hidden="true">
      <span :style="{ width: `${progressPercent}%` }"></span>
    </div>
    <p v-if="batch" class="study-run-meta">
      {{ batch.model || "Gemini Flash-Lite" }} · {{ batch.parallelism || 1 }} parallel ·
      {{ batch.force_rerun ? "rerun" : "cache-aware" }}
    </p>
    <div v-if="failedItems.length" class="study-run-failure">
      <strong>{{ failedItems.length }} copy failed</strong>
      <span>{{ failedItems[0].error }}</span>
      <button type="button" @click="copyText(failureText)">Copy error</button>
    </div>
    <div v-if="items.length" class="study-run-items">
      <div v-for="item in items" :key="`${item.batch_id}:${item.copy_id}`" class="study-run-item">
        <span>{{ item.copy_id }}</span>
        <strong>{{ item.status }}</strong>
        <small>{{ item.error || itemCost(item) }}</small>
        <button v-if="item.error" type="button" @click="copyText(errorTextForItem(item))">Copy</button>
      </div>
    </div>
  </section>
</template>
