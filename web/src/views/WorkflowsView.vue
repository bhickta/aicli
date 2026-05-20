<script setup lang="ts">
import { computed, shallowRef } from "vue";
import ProviderModelControl from "../components/ProviderModelControl.vue";
import DropZone from "../components/workflows/DropZone.vue";
import FileBrowser from "../components/workflows/FileBrowser.vue";
import WorkflowField from "../components/workflows/WorkflowField.vue";
import WorkflowResult from "../components/workflows/WorkflowResult.vue";
import { useWorkflowDrop } from "../composables/useWorkflowDrop";
import { useWorkflowForm } from "../composables/useWorkflowForm";
import { useWorkflowRunner } from "../composables/useWorkflowRunner";
import { api } from "../lib/api";
import { activeWorkflow, activeWorkflowDefinitions, appState, selectWorkflowCategory } from "../stores/appState";
import type { WorkflowField as WorkflowFieldType } from "../types";
import { workflowCategories } from "../workflows/definitions";

const browserField = shallowRef<WorkflowFieldType | null>(null);
const { values, providerModel, nonProviderFields, hasProviderModel, updateField, collectValues } = useWorkflowForm(activeWorkflow);
const { status, result, markdownPreview, sourcePreview, progress, running, runWorkflow: runActiveWorkflow } = useWorkflowRunner();
const { handleDrop } = useWorkflowDrop({ activeWorkflow, updateField, chooseWorkflow, status, result, sourcePreview });
const resourceHint = computed(() => {
  const defaults = appState.systemResources?.defaults;
  if (!defaults) return "";
  if (appState.workflow.category === "Video" && activeWorkflow.value?.id === "video-course") {
    return `Auto workers: ${defaults.video_transcript_workers} transcribe, ${defaults.video_compression_workers} compress`;
  }
  if (appState.workflow.category === "Documents") {
    return `Auto workers: ${defaults.pdf_render_workers} render, ${defaults.ocr_workers} OCR`;
  }
  if (appState.workflow.category === "Zettel") {
    return `Auto workers: ${defaults.zettel_read_workers} note readers`;
  }
  return "";
});

function chooseWorkflow(id: string) {
  appState.workflow.workflowId = id;
}

async function browse(field: WorkflowFieldType) {
  if (field.picker === "directory" && field.id) {
    await pickSystemDirectory(field);
    return;
  }
  browserField.value = field;
}

async function pickSystemDirectory(field: WorkflowFieldType) {
  if (!field.id) return;
  status.value = `Opening system folder picker for ${field.label}...`;
  try {
    const currentPath = String(values[field.id] || appState.browserPath || "");
    const query = currentPath ? `?path=${encodeURIComponent(currentPath)}` : "";
    const picked = await api<{ path: string }>(`/api/fs/pick-directory${query}`);
    updateField(field.id, picked.path);
    appState.browserPath = picked.path;
    status.value = `Selected ${field.label}`;
    if (result.value.includes("For in-place processing")) result.value = "";
  } catch (error) {
    status.value = "Folder picker failed";
    result.value = error instanceof Error ? error.message : "Folder picker failed";
  }
}

async function runWorkflow() {
  await runActiveWorkflow(activeWorkflow.value, collectValues());
}

function handleBrowserSelect(id: string, path: string) {
  updateField(id, path);
  browserField.value = null;
}
</script>

<template>
  <div class="panel grid">
    <h2>Workflows</h2>
    <div class="tabs workflow-category-tabs" role="tablist" aria-label="Workflow categories">
      <button
        v-for="category in workflowCategories"
        :key="category"
        type="button"
        role="tab"
        class="workflow-tab"
        :class="{ 'active-tab': appState.workflow.category === category }"
        :aria-selected="appState.workflow.category === category"
        @click="selectWorkflowCategory(category)"
      >
        {{ category }}
      </button>
    </div>
    <div class="field">
      <label for="workflow-selection">Workflow</label>
      <select id="workflow-selection" :value="appState.workflow.workflowId" @change="chooseWorkflow(($event.target as HTMLSelectElement).value)">
        <option v-for="workflow in activeWorkflowDefinitions" :key="workflow.id" :value="workflow.id">
          {{ workflow.label }}
        </option>
      </select>
    </div>
    <DropZone @upload="handleDrop" />
    <p class="muted">Workflow controls are shown only for the selected workflow. Recall is included in the <strong>Study</strong> workflow set.</p>
    <p v-if="resourceHint" class="status-line compact">{{ resourceHint }}</p>
    <div id="workflow-fields" class="grid">
      <ProviderModelControl
        v-if="hasProviderModel"
        :provider-id="providerModel.provider_id || activeWorkflow?.preferredProviderId || ''"
        :model="providerModel.model || activeWorkflow?.preferredModel || ''"
        @change="Object.assign(providerModel, $event)"
      />
      <WorkflowField
        v-for="field in nonProviderFields"
        :key="field.id"
        :field="field"
        :value="field.id ? values[field.id] : ''"
        @update="updateField"
        @browse="browse"
      />
    </div>
    <div class="field">
      <button id="workflow-run" type="button" :disabled="running" @click="runWorkflow">Run workflow</button>
    </div>
    <WorkflowResult
      :status="status"
      :progress="progress"
      :result="result"
      :source-preview="sourcePreview"
      :markdown-preview="markdownPreview"
    />
    <FileBrowser :field="browserField" @select="handleBrowserSelect" @close="browserField = null" />
  </div>
</template>
