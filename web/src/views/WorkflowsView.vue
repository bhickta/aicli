<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from "vue";
import ProviderModelControl from "../components/ProviderModelControl.vue";
import DropZone from "../components/workflows/DropZone.vue";
import FileBrowser from "../components/workflows/FileBrowser.vue";
import WorkflowField from "../components/workflows/WorkflowField.vue";
import { api, parseJobOutput, pollJob } from "../lib/api";
import { elapsedSeconds, formatDuration, readNumberValue, stringify } from "../lib/format";
import { activeWorkflow, activeWorkflowDefinitions, appState, selectWorkflowCategory } from "../stores/appState";
import type { Job, UploadedFile, WorkflowField as WorkflowFieldType } from "../types";
import { workflowCategories } from "../workflows/definitions";
import type { DropEntry } from "../workflows/drop";

const values = reactive<Record<string, unknown>>({});
const providerModel = reactive({ provider_id: "", model: "" });
const status = shallowRef("Ready");
const result = shallowRef("");
const markdownPreview = shallowRef("");
const sourcePreview = shallowRef("");
const progress = shallowRef(0);
const running = shallowRef(false);
const browserField = shallowRef<WorkflowFieldType | null>(null);

const nonProviderFields = computed(() => activeWorkflow.value?.fields.filter((field) => field.type !== "providerModel") || []);
const hasProviderModel = computed(() => activeWorkflow.value?.fields.some((field) => field.type === "providerModel") || false);

watch(activeWorkflow, initializeValues, { immediate: true });

function initializeValues() {
  for (const key of Object.keys(values)) delete values[key];
  const workflow = activeWorkflow.value;
  if (!workflow) return;
  for (const field of workflow.fields) {
    if (!field.id) continue;
    if (field.type === "path") values[field.id] = appState.workflow.pathValues[field.id] || "";
    else if (field.type === "checkbox") values[field.id] = Boolean(field.checked);
    else if (field.type === "number") values[field.id] = field.default ?? 0;
    else values[field.id] = field.value ?? field.default ?? "";
  }
}

function updateField(id: string, value: unknown) {
  values[id] = value;
  appState.workflow.pathValues[id] = typeof value === "string" ? value : appState.workflow.pathValues[id] || "";
}

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
  const workflow = activeWorkflow.value;
  if (!workflow) return;
  running.value = true;
  status.value = "Running workflow...";
  progress.value = 0;
  result.value = "";
  markdownPreview.value = "";
  try {
    const inputValues = collectValues();
    const response = await api<{ job?: Job; result?: unknown }>(workflow.endpoint, {
      method: "POST",
      body: JSON.stringify(workflow.buildPayload(inputValues)),
    });
    if (response.job?.id && response.job.status === "running") {
      await pollJob(response.job.id, renderWorkflowJob);
      return;
    }
    result.value = stringify(response.result || response);
    status.value = "Completed";
    progress.value = 100;
  } catch (error) {
    status.value = "Failed";
    result.value = error instanceof Error ? error.message : "Workflow failed";
  } finally {
    running.value = false;
  }
}

function collectValues() {
  const out: Record<string, unknown> = { ...values };
  if (hasProviderModel.value) {
    out.provider_id = providerModel.provider_id;
    out.model = providerModel.model;
  }
  for (const field of activeWorkflow.value?.fields || []) {
    if (!field.id || field.type !== "number") continue;
    out[field.id] = readNumberValue(out[field.id], Number(field.default || 0), Number(field.min || 0));
  }
  return out;
}

function renderWorkflowJob(job: Job) {
  const percent = Math.round((job.progress || 0) * 100);
  const eta = job.eta_seconds ? ` | ETA ${formatDuration(job.eta_seconds)}` : "";
  const step = job.total_steps ? ` | ${job.current_step}/${job.total_steps}` : "";
  status.value = `${job.status}: ${job.stage || "working"}${step} | ${percent}% | elapsed ${formatDuration(elapsedSeconds(job.created_at))}${eta}`;
  progress.value = percent;
  if (job.status === "completed") {
    const parsed = parseJobOutput(job.output);
    const maybeMarkdown = parsed && typeof parsed === "object" && "markdown" in parsed ? String((parsed as { markdown?: unknown }).markdown || "") : "";
    markdownPreview.value = maybeMarkdown;
    result.value = parsed ? stringify(parsed) : "";
    status.value = "Completed";
  }
  if (job.status === "failed") {
    result.value = job.error || "Workflow failed";
    status.value = "Failed";
  }
}

async function handleDrop(entries: DropEntry[]) {
  if (entries.some((entry) => entry.relativePath.includes("/"))) {
    selectWorkflowCategory("Video");
    chooseWorkflow("video-course");
    status.value = "Folder not uploaded";
    result.value = "For in-place processing, click Choose Course source folder, navigate to the folder, then use current directory.";
    return;
  }
  const label = entries.length === 1 ? entries[0].file.name : `${entries.length} files`;
  status.value = `Uploading ${label}...`;
  result.value = "";
  try {
    const upload = await uploadEntries(entries);
    const uploaded = upload.files[0];
    if (!uploaded) throw new Error("upload finished without a stored file");
    autoSelectWorkflow(uploaded.name || entries[0].file.name);
    const primaryPathField = activeWorkflow.value?.fields.find((field) => field.type === "path");
    if (primaryPathField?.id) updateField(primaryPathField.id, uploaded.path);
    sourcePreview.value = uploaded.url || "";
    status.value = `Ready: ${uploaded.name || entries[0].file.name}`;
  } catch (error) {
    status.value = "Upload failed";
    result.value = error instanceof Error ? error.message : "Upload failed";
  }
}

async function uploadEntries(entries: DropEntry[]) {
  const form = new FormData();
  entries.forEach((entry) => {
    const fieldName = entry.relativePath.includes("/") ? `file:${entry.relativePath}` : "file";
    form.append(fieldName, entry.file, entry.file.name);
  });
  const response = await fetch("/api/fs/upload", { method: "POST", body: form });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json() as Promise<{ root: string; files: UploadedFile[] }>;
}

function autoSelectWorkflow(fileName: string) {
  const lower = fileName.toLowerCase();
  if (lower.endsWith(".pdf")) {
    selectWorkflowCategory("Documents");
    chooseWorkflow("ocr-pdf");
  } else if (lower.endsWith(".zip")) {
    selectWorkflowCategory("Documents");
    chooseWorkflow("ocr-run");
  } else if (/\.(png|jpe?g|webp|gif|bmp|tiff?)$/.test(lower)) {
    selectWorkflowCategory("Images");
    chooseWorkflow("image-run");
  } else if (/\.(mp3|wav|m4a|flac|ogg|opus)$/.test(lower)) {
    selectWorkflowCategory("Audio");
    chooseWorkflow("audio-transcribe");
  } else if (/\.(mp4|mov|mkv|webm|avi|m4v)$/.test(lower)) {
    selectWorkflowCategory("Video");
    chooseWorkflow("video-course");
  }
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
    <div id="workflow-fields" class="grid">
      <ProviderModelControl
        v-if="hasProviderModel"
        :provider-id="activeWorkflow?.preferredProviderId || ''"
        :model="activeWorkflow?.preferredModel || ''"
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
    <div class="field">
      <h3>Status</h3>
      <p id="workflow-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div id="workflow-progress" class="progress" :class="{ hidden: progress <= 0 }">
      <div :style="{ width: `${Math.max(0, Math.min(100, progress))}%` }" />
    </div>
    <div class="field">
      <h3>Result</h3>
      <pre id="workflow-result" role="status" aria-live="polite">{{ result }}</pre>
    </div>
    <div id="review-pane" class="review-pane" :class="{ hidden: !sourcePreview && !markdownPreview }">
      <iframe id="source-preview" title="Source file preview" :src="sourcePreview || undefined" />
      <textarea id="markdown-preview" :value="markdownPreview" readonly placeholder="Markdown preview appears here" />
    </div>
    <FileBrowser :field="browserField" @select="handleBrowserSelect" @close="browserField = null" />
  </div>
</template>
