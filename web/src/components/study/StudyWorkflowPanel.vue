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
    <div class="study-workflow-toolbar">
      <WorkflowSelector
        :workflows="studyWorkflowDefinitions"
        :selected-id="activeWorkflow?.id || ''"
        @select="chooseWorkflow"
      />
      <WorkflowSoundControl />
    </div>

    <div class="study-workflow-layout">
      <section class="study-workflow-controls">
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
      </section>

      <section class="study-workflow-result">
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
      </section>
    </div>
    <FileBrowser :field="browserField" @select="handleBrowserSelect" @close="closeBrowser" />
  </section>
</template>

<style scoped>
.study-workflow-panel {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.study-workflow-toolbar {
  align-items: center;
  background: #0f141c;
  border: 1px solid #2b3440;
  border-radius: 7px;
  display: flex;
  gap: 10px;
  justify-content: space-between;
  min-width: 0;
  padding: 8px;
}

.study-workflow-layout {
  align-items: start;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(20rem, 24rem) minmax(0, 1fr);
  min-width: 0;
}

.study-workflow-controls,
.study-workflow-result {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.study-workflow-controls {
  background: #0d121b;
  border: 1px solid #253247;
  border-radius: 7px;
  padding: 8px;
}

.study-workflow-toolbar :deep(.workflow-selector) {
  min-width: 0;
}

.study-workflow-controls :deep(.drop-zone) {
  min-height: 5.25rem;
  padding: 12px;
}

.study-workflow-controls :deep(.workflow-fields) {
  gap: 8px;
}

.study-workflow-controls :deep(.workflow-run-controls) {
  gap: 8px;
}

@media (max-width: 980px) {
  .study-workflow-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .study-workflow-toolbar {
    align-items: stretch;
    display: grid;
  }
}
</style>
