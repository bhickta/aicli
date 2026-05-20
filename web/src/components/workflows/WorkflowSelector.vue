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
  <div class="field">
    <label for="workflow-selection">Workflow</label>
    <select id="workflow-selection" :value="selectedId" @change="emit('select', ($event.target as HTMLSelectElement).value)">
      <option v-for="workflow in workflows" :key="workflow.id" :value="workflow.id">
        {{ workflow.label }}
      </option>
    </select>
  </div>
</template>
