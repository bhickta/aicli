<script setup lang="ts">
import ZettelSection from "../common/ZettelSection.vue";

defineProps<{
  status: string;
  busy: boolean;
  progressClass: Record<string, boolean>;
  progressStyle: Record<string, string>;
  rawResultSummary: string;
  result: string;
}>();

const emit = defineEmits<{
  rollback: [];
}>();
</script>

<template>
  <ZettelSection title="Status">
    <template #actions>
      <button type="button" :disabled="busy" @click="emit('rollback')">Rollback</button>
    </template>
    <p class="status-line" role="status" aria-live="polite">{{ status }}</p>
    <div class="progress" :class="progressClass">
      <div :style="progressStyle" />
    </div>
    <details v-if="rawResultSummary" class="zettel-raw-result">
      <summary>{{ rawResultSummary }}</summary>
      <pre role="status" aria-live="polite">{{ result }}</pre>
    </details>
  </ZettelSection>
</template>

<style scoped>
.zettel-raw-result {
  display: grid;
  gap: 8px;
}

.zettel-raw-result summary {
  cursor: pointer;
  color: #d6deea;
}
</style>
