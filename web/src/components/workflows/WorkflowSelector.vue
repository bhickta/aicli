<script setup lang="ts">
import type { WorkflowDefinition } from "../../types";

defineProps<{
  workflows: WorkflowDefinition[];
  selectedId: string;
}>();

const emit = defineEmits<{
  select: [workflowId: string];
}>();
</script>

<template>
  <div class="workflow-selector">
    <span class="workflow-selector-label">Workflow</span>
    <div class="workflow-selector-options" role="group" aria-label="Workflow">
      <button
        v-for="workflow in workflows"
        :key="workflow.id"
        type="button"
        class="workflow-selector-option"
        :class="{ active: selectedId === workflow.id }"
        :aria-pressed="selectedId === workflow.id"
        @click="emit('select', workflow.id)"
      >
        {{ workflow.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.workflow-selector {
  display: grid;
  gap: 0.45rem;
}

.workflow-selector-label {
  color: #cbd5e1;
  font-size: 0.82rem;
  font-weight: 600;
}

.workflow-selector-options {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.workflow-selector-option {
  min-height: 2.1rem;
  border: 1px solid #334155;
  border-radius: 0.45rem;
  background: #111827;
  color: #dbeafe;
  cursor: pointer;
  font: inherit;
  padding: 0.4rem 0.7rem;
}

.workflow-selector-option:hover {
  border-color: #60a5fa;
}

.workflow-selector-option.active {
  border-color: #60a5fa;
  background: #1e3a5f;
  color: #ffffff;
}
</style>
