<script setup lang="ts">
import { computed, shallowRef, watch } from "vue";
import DropZone from "../workflows/DropZone.vue";
import FileBrowser from "../workflows/FileBrowser.vue";
import WorkflowFields from "../workflows/WorkflowFields.vue";
import WorkflowResult from "../workflows/WorkflowResult.vue";
import WorkflowRunControls from "../workflows/WorkflowRunControls.vue";
import WorkflowSelector from "../workflows/WorkflowSelector.vue";
import WorkflowSoundControl from "../workflows/WorkflowSoundControl.vue";
import { useWorkflowBrowser } from "../../composables/useWorkflowBrowser";
import { useWorkflowDrop } from "../../composables/useWorkflowDrop";
import { useWorkflowForm } from "../../composables/useWorkflowForm";
import { useWorkflowRunner } from "../../composables/useWorkflowRunner";
import { readStoredString, writeStoredString } from "../../lib/persistence";
import { studyWorkflowDefinitions } from "../../workflows/study";

const selectedWorkflowId = shallowRef(readStoredString("aicli.study.workflow.id", "analyze"));
const activeWorkflow = computed(() => {
  return studyWorkflowDefinitions.find((workflow) => workflow.id === selectedWorkflowId.value) || studyWorkflowDefinitions[0];
});

const { values, providerModel, nonProviderFields, hasProviderModel, updateField, collectValues } = useWorkflowForm(activeWorkflow);
const {
  status,
  result,
  parsedResult,
  markdownPreview,
  sourcePreview,
  progress,
  progressMode,
  progressVisible,
  running,
  currentJob,
  runWorkflow: runActiveWorkflow,
  cancelWorkflow,
} = useWorkflowRunner();
const { handleDrop } = useWorkflowDrop({ activeWorkflow, updateField, chooseWorkflow, status, result, sourcePreview });
const { browserField, browse, handleBrowserSelect, closeBrowser } = useWorkflowBrowser({ values, updateField, status, result });

watch(selectedWorkflowId, (workflowId) => writeStoredString("aicli.study.workflow.id", workflowId));

function chooseWorkflow(id: string) {
  selectedWorkflowId.value = id;
}

async function runWorkflow() {
  await runActiveWorkflow(activeWorkflow.value, collectValues());
}
</script>

<template>
  <section class="study-workflow-panel">
    <header class="study-workflow-header">
      <div>
        <h3>Study Workflows</h3>
        <p class="muted">Run recall, OCR, and topper answer-copy analysis from the Study area.</p>
      </div>
      <WorkflowSoundControl />
    </header>

    <WorkflowSelector
      :workflows="studyWorkflowDefinitions"
      :selected-id="activeWorkflow?.id || ''"
      @select="chooseWorkflow"
    />
    <DropZone @upload="handleDrop" />
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
      cancel-label="Cancel workflow"
      @run="runWorkflow"
      @cancel="cancelWorkflow"
    />
    <WorkflowResult
      :status="status"
      :progress="progress"
      :progress-mode="progressMode"
      :progress-visible="progressVisible"
      :result="result"
      :parsed-result="parsedResult"
      :source-preview="sourcePreview"
      :markdown-preview="markdownPreview"
    />
    <FileBrowser :field="browserField" @select="handleBrowserSelect" @close="closeBrowser" />
  </section>
</template>

<style scoped>
.study-workflow-panel {
  display: grid;
  gap: 12px;
}

.study-workflow-header {
  align-items: flex-start;
  display: flex;
  gap: 12px;
  justify-content: space-between;
}

.study-workflow-header h3,
.study-workflow-header p {
  margin: 0;
}

@media (max-width: 760px) {
  .study-workflow-header {
    display: grid;
  }
}
</style>
