<script setup lang="ts">
import { computed, shallowRef } from "vue";
import type { WorkflowDefinition } from "../../types";

const props = defineProps<{
  workflows: WorkflowDefinition[];
  selectedId: string;
}>();

const emit = defineEmits<{
  select: [workflowId: string];
}>();

const query = shallowRef("");

const selectedWorkflow = computed(() => {
  return props.workflows.find((workflow) => workflow.id === props.selectedId);
});

const visibleWorkflows = computed(() => {
  const needle = query.value.trim().toLowerCase();
  if (!needle) return props.workflows;
  return props.workflows.filter((workflow) => {
    return [
      workflow.label,
      workflow.category,
      workflow.id,
      workflow.fields.map((field) => field.label || field.id || field.type).join(" "),
    ]
      .join(" ")
      .toLowerCase()
      .includes(needle);
  });
});

function workflowMeta(workflow: WorkflowDefinition) {
  const providerSteps = workflow.fields.filter((field) => field.type === "providerModel" || field.type === "stepProviderModel").length;
  const pathFields = workflow.fields.filter((field) => field.type === "path").length;
  const parts = [`${workflow.fields.length} fields`];
  if (providerSteps) parts.push(`${providerSteps} model${providerSteps === 1 ? "" : "s"}`);
  if (pathFields) parts.push(`${pathFields} path${pathFields === 1 ? "" : "s"}`);
  return parts.join(" · ");
}
</script>

<template>
  <div class="workflow-selector">
    <div class="workflow-selector-header">
      <div>
        <span class="workflow-selector-label">Workflow</span>
        <strong v-if="selectedWorkflow">{{ selectedWorkflow.label }}</strong>
      </div>
      <span>{{ visibleWorkflows.length }}/{{ workflows.length }}</span>
    </div>
    <input
      v-model="query"
      type="search"
      class="workflow-selector-search"
      placeholder="Search workflows, categories, fields..."
      aria-label="Search workflows"
    >
    <div class="workflow-selector-options" role="listbox" aria-label="Workflow">
      <button
        v-for="workflow in visibleWorkflows"
        :key="workflow.id"
        type="button"
        class="workflow-selector-option"
        :class="{ active: selectedId === workflow.id }"
        role="option"
        :aria-selected="selectedId === workflow.id"
        @click="emit('select', workflow.id)"
      >
        <span class="workflow-option-label">{{ workflow.label }}</span>
        <span class="workflow-option-meta">{{ workflow.category }} · {{ workflowMeta(workflow) }}</span>
      </button>
      <p v-if="!visibleWorkflows.length" class="workflow-selector-empty">No workflows match this search.</p>
    </div>
  </div>
</template>

<style scoped>
.workflow-selector {
  display: grid;
  gap: 0.5rem;
  min-width: 0;
}

.workflow-selector-header {
  align-items: end;
  display: flex;
  gap: 0.75rem;
  justify-content: space-between;
  min-width: 0;
}

.workflow-selector-header div {
  display: grid;
  gap: 0.1rem;
  min-width: 0;
}

.workflow-selector-header strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workflow-selector-header > span {
  color: #94a3b8;
  flex: none;
  font-size: 0.78rem;
}

.workflow-selector-label {
  color: #cbd5e1;
  font-size: 0.82rem;
  font-weight: 600;
}

.workflow-selector-search {
  width: 100%;
}

.workflow-selector-options {
  display: grid;
  gap: 0.4rem;
  max-height: 22rem;
  min-width: 0;
  overflow: auto;
}

.workflow-selector-option {
  border: 1px solid #334155;
  border-radius: 0.4rem;
  background: #111827;
  color: #dbeafe;
  cursor: pointer;
  display: grid;
  font: inherit;
  gap: 0.18rem;
  min-height: 3.25rem;
  min-width: 0;
  padding: 0.45rem 0.6rem;
  text-align: left;
}

.workflow-selector-option:hover {
  border-color: #60a5fa;
}

.workflow-selector-option.active {
  border-color: #60a5fa;
  background: #1e3a5f;
  color: #ffffff;
}

.workflow-option-label {
  font-weight: 650;
  line-height: 1.2;
}

.workflow-option-meta {
  color: #9fb0c6;
  font-size: 0.76rem;
  line-height: 1.25;
}

.workflow-selector-option.active .workflow-option-meta {
  color: #d6e7ff;
}

.workflow-selector-empty {
  border: 1px dashed #334155;
  border-radius: 0.4rem;
  color: #94a3b8;
  margin: 0;
  padding: 0.7rem;
}
</style>
