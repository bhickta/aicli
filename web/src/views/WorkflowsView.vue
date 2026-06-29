<script setup lang="ts">
import { computed, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import DropZone from "../components/workflows/DropZone.vue";
import FileBrowser from "../components/workflows/FileBrowser.vue";
import WorkflowCategoryTabs from "../components/workflows/WorkflowCategoryTabs.vue";
import WorkflowFields from "../components/workflows/WorkflowFields.vue";
import WorkflowRunControls from "../components/workflows/WorkflowRunControls.vue";
import WorkflowSelector from "../components/workflows/WorkflowSelector.vue";
import WorkflowSoundControl from "../components/workflows/WorkflowSoundControl.vue";
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
const resourceHint = useWorkflowResourceHint();
const route = useRoute();
const router = useRouter();

const routeCategory = computed(() => normalizeRouteCategory(String(route.params.category || "")));

watch(routeCategory, (category) => {
  if (appState.workflow.category !== category) {
    appState.workflow.category = category;
    appState.workflow.workflowId = activeWorkflowDefinitions.value[0]?.id || "";
  }
}, { immediate: true });

function chooseWorkflow(id: string) {
  appState.workflow.workflowId = id;
}

function chooseCategory(category: string) {
  if (category === "Zettel") {
    void router.push({ name: "zettel", params: { mode: "inbox" } });
    return;
  }
  selectWorkflowCategory(category);
  void router.push({ name: "workflows", params: { category: category.toLowerCase() } });
}

async function runWorkflow() {
  await runActiveWorkflow(activeWorkflow.value, collectValues());
}

function cancelButtonLabel() {
  return activeWorkflow.value?.id === "whatsapp-schedule" ? "Cancel schedule" : "Cancel workflow";
}

function normalizeRouteCategory(category: string) {
  const normalized = workflowCategories.find((item) => item.toLowerCase() === category.toLowerCase());
  if (normalized && normalized !== "Zettel") return normalized;
  return appState.workflow.category === "Zettel" ? "Codex" : appState.workflow.category;
}
</script>

<template>
  <div class="panel workflows-page">
    <section class="workflows-rail">
      <div class="workflows-heading">
        <h2>Workflows</h2>
        <p class="muted">Pick a category, then search within its tools.</p>
      </div>
      <WorkflowCategoryTabs
        :categories="workflowCategories"
        :active-category="appState.workflow.category"
        @select="chooseCategory"
      />
      <WorkflowSelector
        :workflows="activeWorkflowDefinitions"
        :selected-id="activeWorkflow?.id || ''"
        @select="chooseWorkflow"
      />
    </section>

    <section class="workflows-workbench">
      <header class="workflows-active-header">
        <div>
          <span>{{ appState.workflow.category }}</span>
          <h3>{{ activeWorkflow?.label || "Select a workflow" }}</h3>
        </div>
        <WorkflowSoundControl />
      </header>
      <DropZone @upload="handleDrop" />
      <p v-if="resourceHint" class="status-line compact">{{ resourceHint }}</p>
      <section class="workflow-run-card">
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
      </section>
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
  </div>
</template>
