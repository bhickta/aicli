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
import "../../styles/study-workflow.css";
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
const { handleDrop } = useWorkflowDrop({
  activeWorkflow,
  updateField,
  chooseWorkflow,
  status,
  result,
  sourcePreview,
  autoSelectWorkflow: autoSelectStudyWorkflow,
});
const { browserField, browse, handleBrowserSelect, closeBrowser } = useWorkflowBrowser({ values, updateField, status, result });

watch(selectedWorkflowId, (workflowId) => writeStoredString("aicli.study.workflow.id", workflowId));

function chooseWorkflow(id: string) {
  selectedWorkflowId.value = id;
}

function autoSelectStudyWorkflow(fileName: string) {
  if (fileName.toLowerCase().endsWith(".pdf")) {
    chooseWorkflow("analyze");
  }
}

async function runWorkflow() {
  await runActiveWorkflow(activeWorkflow.value, collectValues());
}
</script>

<template>
  <section class="study-workflow-panel">
    <aside class="study-workflow-rail" aria-label="Study workflows">
      <WorkflowSelector
        :workflows="studyWorkflowDefinitions"
        :selected-id="activeWorkflow?.id || ''"
        @select="chooseWorkflow"
      />
      <WorkflowSoundControl />
    </aside>

    <section class="study-workflow-controls" aria-label="Workflow inputs">
      <header class="study-run-header">
        <div>
          <span>Selected workflow</span>
          <h3>{{ activeWorkflow?.label }}</h3>
        </div>
        <span>{{ nonProviderFields.length }} input{{ nonProviderFields.length === 1 ? "" : "s" }}</span>
      </header>
      <div class="study-input-scroll">
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
      </div>
      <footer class="study-run-footer">
        <WorkflowRunControls
          :running="running"
          :can-cancel="Boolean(currentJob)"
          cancel-label="Cancel workflow"
          @run="runWorkflow"
          @cancel="cancelWorkflow"
        />
      </footer>
    </section>

    <section class="study-workflow-result" aria-label="Workflow output">
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
    <FileBrowser :field="browserField" @select="handleBrowserSelect" @close="closeBrowser" />
  </section>
</template>
