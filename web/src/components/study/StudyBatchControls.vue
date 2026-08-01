<script setup lang="ts">
import { computed } from "vue";
import ProviderModelControl from "../ProviderModelControl.vue";
import { providers } from "../../stores/appState";

defineProps<{
  selectedCount: number;
  providerId: string;
  model: string;
  parallelism: number;
  forceRerun: boolean;
  running: boolean;
}>();

const emit = defineEmits<{
  "update:providerId": [value: string];
  "update:model": [value: string];
  "update:parallelism": [value: number];
  "update:forceRerun": [value: boolean];
  runSelected: [];
  generateMetadata: [];
  clear: [];
}>();

const lmStudioProviders = computed(() => providers.value.filter((provider) => provider.id === "lms"));

function updateModel(value: { provider_id: string; model: string }) {
  emit("update:providerId", value.provider_id);
  emit("update:model", value.model);
}
</script>

<template>
  <section class="study-batch-controls" aria-label="Batch analysis controls">
    <div class="study-local-model">
      <strong>LM Studio local analysis</strong>
      <ProviderModelControl
        :provider-id="providerId"
        :model="model"
        :provider-options="lmStudioProviders"
        @change="updateModel"
      />
      <p>Load a vision-capable model in LM Studio. Empty selection uses the first model currently loaded by the local server.</p>
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
        <span>Parallel</span>
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
</style>
