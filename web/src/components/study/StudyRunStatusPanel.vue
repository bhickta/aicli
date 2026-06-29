<script setup lang="ts">
import { computed } from "vue";
import type { StudyBatchItemRecord, StudyBatchRecord } from "../../types";

const props = defineProps<{
  batch: StudyBatchRecord | null;
  items: StudyBatchItemRecord[];
  status: string;
}>();

const progressText = computed(() => {
  if (!props.batch) return props.status || "No active run";
  return `${props.batch.completed}/${props.batch.total} complete, ${props.batch.failed} failed`;
});

function itemCost(item: StudyBatchItemRecord) {
  if (item.cache_hit) return "cached";
  if (!item.api_calls && !item.total_tokens) return "";
  return `${item.api_calls || 0} call(s), ${item.total_tokens || 0} tokens`;
}
</script>

<template>
  <section v-if="batch || status" class="study-run-status" aria-label="Analysis run status">
    <header>
      <div>
        <span>Current run</span>
        <strong>{{ batch?.status || status }}</strong>
      </div>
      <span>{{ progressText }}</span>
    </header>
    <div v-if="items.length" class="study-run-items">
      <div v-for="item in items" :key="`${item.batch_id}:${item.copy_id}`" class="study-run-item">
        <span>{{ item.copy_id }}</span>
        <strong>{{ item.status }}</strong>
        <small>{{ item.error || itemCost(item) }}</small>
      </div>
    </div>
  </section>
</template>
