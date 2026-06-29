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
import { api } from "../../lib/api";
import { readStoredString, writeStoredString } from "../../lib/persistence";
import "../../styles/study-workflow.css";
import { studyWorkflowDefinitions } from "../../workflows/study";

const props = withDefaults(defineProps<{
  compact?: boolean;
  lockedWorkflowId?: string;
  reviewId?: string;
  syncCopyId?: string;
  sourcePath?: string;
}>(), {
  compact: false,
  lockedWorkflowId: "",
  reviewId: "",
  syncCopyId: "",
  sourcePath: "",
});
const emit = defineEmits<{ synced: [] }>();

const selectedWorkflowId = shallowRef(props.lockedWorkflowId || readStoredString("aicli.study.workflow.id", "analyze"));
const activeWorkflow = computed(() => {
  const id = props.lockedWorkflowId || selectedWorkflowId.value;
  return studyWorkflowDefinitions.find((workflow) => workflow.id === id) || studyWorkflowDefinitions[0];
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
const showResult = computed(() => !props.compact || progressVisible.value || Boolean(result.value || parsedResult.value));

watch(selectedWorkflowId, (workflowId) => {
  if (!props.lockedWorkflowId) writeStoredString("aicli.study.workflow.id", workflowId);
});
watch(() => props.lockedWorkflowId, (workflowId) => {
  if (workflowId) selectedWorkflowId.value = workflowId;
});
watch([activeWorkflow, () => props.sourcePath], ([workflow, sourcePath]) => {
  if (workflow?.id === "analyze" && sourcePath && values.path !== sourcePath) updateField("path", sourcePath);
}, { immediate: true });

function chooseWorkflow(id: string) {
  if (props.lockedWorkflowId) return;
  selectedWorkflowId.value = id;
}

function autoSelectStudyWorkflow(fileName: string) {
  if (fileName.toLowerCase().endsWith(".pdf")) {
    chooseWorkflow("analyze");
  }
}

async function runWorkflow() {
  const inputValues = collectValues();
  if (props.reviewId) inputValues.review_id = props.reviewId;
  await runActiveWorkflow(activeWorkflow.value, inputValues);
  await syncStudyCopyAfterRun();
}

async function syncStudyCopyAfterRun() {
  if (!props.syncCopyId || !isTopperCopyReview(parsedResult.value)) return;
  status.value = "Saving study copy...";
  try {
    await api(`/api/study/copies/${encodeURIComponent(props.syncCopyId)}/sync`, {
      method: "POST",
      body: JSON.stringify({
        review: parsedResult.value,
        source_path: props.sourcePath,
        provider_id: providerModel.provider_id,
        model: providerModel.model,
      }),
    });
    status.value = "Saved to study copy";
    emit("synced");
  } catch (error) {
    status.value = "Save failed";
    result.value = error instanceof Error ? error.message : "Study copy sync failed";
  }
}

function isTopperCopyReview(value: unknown) {
  return Boolean(value && typeof value === "object" && (value as { kind?: unknown }).kind === "topper_copy_review");
}
</script>

<template>
  <section class="study-workflow-panel" :class="{ compact, 'with-result': showResult }">
    <aside v-if="!compact && !lockedWorkflowId" class="study-workflow-rail" aria-label="Study workflows">
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
        <DropZone v-if="!compact" @upload="handleDrop" />
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

    <section v-if="showResult" class="study-workflow-result" aria-label="Workflow output">
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
