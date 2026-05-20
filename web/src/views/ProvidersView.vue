<script setup lang="ts">
import { computed, shallowRef, watch } from "vue";
import { api } from "../lib/api";
import { stringify } from "../lib/format";
import { appState, defaultProviderId, providers } from "../stores/appState";
import type { Model } from "../types";

const selectedProvider = shallowRef(defaultProviderId.value);
const status = shallowRef("Loading providers...");
const models = shallowRef<Model[]>([]);
const busy = shallowRef(false);

const providerConfig = computed(() => stringify(appState.settings?.providers || []));
const modelResponse = computed(() => stringify(models.value));

watch(defaultProviderId, (id) => {
  if (!selectedProvider.value) selectedProvider.value = id;
}, { immediate: true });

watch(selectedProvider, () => {
  void loadModels();
}, { immediate: true });

async function loadModels() {
  if (!selectedProvider.value) {
    status.value = "Select a provider.";
    models.value = [];
    return;
  }
  busy.value = true;
  status.value = `Loading models for ${selectedProvider.value}...`;
  try {
    const payload = await api<{ models: Model[] }>(`/api/providers/${encodeURIComponent(selectedProvider.value)}/models`);
    models.value = payload.models || [];
    appState.models[selectedProvider.value] = models.value;
    status.value = "Models loaded";
  } catch (error) {
    models.value = [];
    status.value = "Failed to load models";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="panel grid">
    <h2>Providers</h2>
    <div class="field">
      <label for="provider-list">Provider</label>
      <select id="provider-list" v-model="selectedProvider">
        <option v-for="provider in providers" :key="provider.id" :value="provider.id">
          {{ provider.name || provider.id }}
        </option>
      </select>
    </div>
    <div class="field">
      <button id="load-models" type="button" :disabled="busy" @click="loadModels">Load models</button>
    </div>
    <div class="field">
      <h3>Status</h3>
      <p id="providers-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div class="field">
      <h3>Model response</h3>
      <pre id="providers-models" role="status" aria-live="polite">{{ modelResponse }}</pre>
    </div>
    <div class="field">
      <h3>Configured providers</h3>
      <pre id="provider-config" role="status" aria-live="polite">{{ providerConfig }}</pre>
    </div>
  </div>
</template>
