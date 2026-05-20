<script setup lang="ts">
import DropZone from "../components/workflows/DropZone.vue";
import FileBrowser from "../components/workflows/FileBrowser.vue";
import WorkflowCategoryTabs from "../components/workflows/WorkflowCategoryTabs.vue";
import WorkflowFields from "../components/workflows/WorkflowFields.vue";
import WorkflowRunControls from "../components/workflows/WorkflowRunControls.vue";
import WorkflowSelector from "../components/workflows/WorkflowSelector.vue";
import WorkflowResult from "../components/workflows/WorkflowResult.vue";
import { useWorkflowBrowser } from "../composables/useWorkflowBrowser";
import { useWorkflowDrop } from "../composables/useWorkflowDrop";
import { useWorkflowForm } from "../composables/useWorkflowForm";
import { useWorkflowResourceHint } from "../composables/useWorkflowResourceHint";
import { useWorkflowRunner } from "../composables/useWorkflowRunner";
import { activeWorkflow, activeWorkflowDefinitions, appState, selectWorkflowCategory } from "../stores/appState";
import { workflowCategories } from "../workflows/definitions";

const { values, providerModel, nonProviderFields, hasProviderModel, updateField, collectValues } = useWorkflowForm(activeWorkflow);
const {
  status,
  result,
  markdownPreview,
  sourcePreview,
  progress,
  running,
  currentJob,
  runWorkflow: runActiveWorkflow,
  cancelWorkflow,
} = useWorkflowRunner();
const { handleDrop } = useWorkflowDrop({ activeWorkflow, updateField, chooseWorkflow, status, result, sourcePreview });
const { browserField, browse, handleBrowserSelect, closeBrowser } = useWorkflowBrowser({ values, updateField, status, result });
const resourceHint = useWorkflowResourceHint();

function chooseWorkflow(id: string) {
  appState.workflow.workflowId = id;
}

async function runWorkflow() {
  await runActiveWorkflow(activeWorkflow.value, collectValues());
}

function cancelButtonLabel() {
  return activeWorkflow.value?.id === "whatsapp-schedule" ? "Cancel schedule" : "Cancel workflow";
}
</script>

<template>
  <div class="panel grid">
    <h2>Workflows</h2>
    <WorkflowCategoryTabs
      :categories="workflowCategories"
      :active-category="appState.workflow.category"
      @select="selectWorkflowCategory"
    />
    <WorkflowSelector
      :workflows="activeWorkflowDefinitions"
      :selected-id="appState.workflow.workflowId"
      @select="chooseWorkflow"
    />
    <DropZone @upload="handleDrop" />
    <p class="muted">Workflow controls are shown only for the selected workflow. Recall is included in the <strong>Study</strong> workflow set.</p>
    <p v-if="resourceHint" class="status-line compact">{{ resourceHint }}</p>
    <WorkflowFields
      :fields="nonProviderFields"
      :values="values"
      :has-provider-model="hasProviderModel"
      :provider-model="providerModel"
      :preferred-provider-id="activeWorkflow?.preferredProviderId"
      :preferred-model="activeWorkflow?.preferredModel"
      @update-field="updateField"
      @update-provider-model="Object.assign(providerModel, $event)"
      @browse="browse"
    />
    <WorkflowRunControls
      :running="running"
      :can-cancel="Boolean(currentJob)"
      :cancel-label="cancelButtonLabel()"
      @run="runWorkflow"
      @cancel="cancelWorkflow"
    />
    <WorkflowResult
      :status="status"
      :progress="progress"
      :result="result"
      :source-preview="sourcePreview"
      :markdown-preview="markdownPreview"
    />
    <FileBrowser :field="browserField" @select="handleBrowserSelect" @close="closeBrowser" />
  </div>
</template>
