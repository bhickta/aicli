<script setup lang="ts">
import { computed } from "vue";
import ProviderModelControl from "../ProviderModelControl.vue";
import { providers } from "../../stores/appState";

defineProps<{
  selectedCount: number;
  providerId: string;
  ocrModel: string;
  questionModel: string;
  boundaryModel: string;
  reportModel: string;
  parallelism: number;
  forceRerun: boolean;
  running: boolean;
}>();

const emit = defineEmits<{
  "update:providerId": [value: string];
  "update:ocrModel": [value: string];
  "update:questionModel": [value: string];
  "update:boundaryModel": [value: string];
  "update:reportModel": [value: string];
  "update:parallelism": [value: number];
  "update:forceRerun": [value: boolean];
  runSelected: [];
  generateMetadata: [];
  clear: [];
}>();

const lmStudioProviders = computed(() => providers.value.filter((provider) => provider.id === "lms"));

function updateModel(stage: "ocr" | "question" | "boundary" | "report", value: { provider_id: string; model: string }) {
  emit("update:providerId", value.provider_id);
  if (stage === "ocr") emit("update:ocrModel", value.model);
  if (stage === "question") emit("update:questionModel", value.model);
  if (stage === "boundary") emit("update:boundaryModel", value.model);
  if (stage === "report") emit("update:reportModel", value.model);
}
</script>

<template>
  <section class="study-batch-controls" aria-label="Batch analysis controls">
    <div class="study-local-model">
      <strong>LM Studio local analysis</strong>
      <label class="study-model-stage">
        <span>Handwriting OCR</span>
        <ProviderModelControl
          :provider-id="providerId"
          :model="ocrModel"
          :provider-options="lmStudioProviders"
          @change="updateModel('ocr', $event)"
        />
      </label>
      <label class="study-model-stage">
        <span>Question analysis</span>
        <ProviderModelControl
          :provider-id="providerId"
          :model="questionModel"
          :provider-options="lmStudioProviders"
          @change="updateModel('question', $event)"
        />
      </label>
      <label class="study-model-stage">
        <span>Answer boundaries</span>
        <ProviderModelControl
          :provider-id="providerId"
          :model="boundaryModel"
          :provider-options="lmStudioProviders"
          @change="updateModel('boundary', $event)"
        />
      </label>
      <label class="study-model-stage">
        <span>Final synthesis</span>
        <ProviderModelControl
          :provider-id="providerId"
          :model="reportModel"
          :provider-options="lmStudioProviders"
          @change="updateModel('report', $event)"
        />
      </label>
      <p>When OCR and analysis use different models, the batch saves OCR, unloads that model, and resumes with the reasoning model.</p>
    </div>
    <div class="study-batch-row">
      <button type="button" :disabled="running" @click="emit('clear')">New Import / Run</button>
      <button type="button" :disabled="running || !selectedCount" @click="emit('runSelected')">
        {{ running ? "Running..." : "Run selected" }}
      </button>
      <button type="button" :disabled="running || !selectedCount" @click="emit('generateMetadata')">
        Metadata only
      </button>
    </div>
    <div class="study-batch-row">
      <label class="study-parallel-control">
        <span>Parallel model calls</span>
        <input
          :value="parallelism"
          :disabled="running"
          type="number"
          min="1"
          max="5"
          @input="emit('update:parallelism', Number(($event.target as HTMLInputElement).value || 1))"
        />
      </label>
      <label class="study-rerun-control">
        <input
          type="checkbox"
          :checked="forceRerun"
          :disabled="running"
          @change="emit('update:forceRerun', ($event.target as HTMLInputElement).checked)"
        />
        <span>Bypass cache</span>
      </label>
    </div>
    <p class="study-parallel-help">
      This is one shared budget: a single copy uses it across pages and questions; batches divide it across copies.
    </p>
  </section>
</template>

<style scoped>
.study-local-model {
  background: rgba(56, 139, 253, 0.05);
  border: 1px solid rgba(56, 139, 253, 0.18);
  border-radius: 8px;
  display: grid;
  gap: 8px;
  padding: 10px;
}

.study-local-model > strong {
  color: #e6edf3;
  font-size: 0.8rem;
}

.study-local-model p {
  color: #8b949e;
  font-size: 0.7rem;
  line-height: 1.4;
  margin: 0;
}

.study-model-stage {
  display: grid;
  gap: 4px;
}

.study-model-stage > span {
  color: #aebdd0;
  font-size: 0.7rem;
  font-weight: 600;
}

.study-parallel-help {
  color: #8b949e;
  font-size: 0.7rem;
  line-height: 1.4;
  margin: 0;
}
</style>
