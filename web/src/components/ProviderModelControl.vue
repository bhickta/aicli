<script setup lang="ts">
import { computed, shallowRef, watch } from "vue";
import { api } from "../lib/api";
import { appState, defaultModel, defaultProviderId, providers } from "../stores/appState";
import type { Model } from "../types";

const props = withDefaults(defineProps<{
  providerId?: string;
  model?: string;
  includeRefresh?: boolean;
}>(), {
  providerId: "",
  model: "",
  includeRefresh: true,
});

const emit = defineEmits<{
  change: [value: { provider_id: string; model: string }];
}>();

const selectedProvider = shallowRef(props.providerId || defaultProviderId.value);
const selectedProviderConfig = computed(() => providers.value.find((provider) => provider.id === selectedProvider.value));
const providerDefaultModel = computed(() => selectedProviderConfig.value?.model || defaultModel.value || "");
const selectedModel = shallowRef(props.model || providerDefaultModel.value);
const status = shallowRef("");
const loading = shallowRef(false);

const models = computed(() => appState.models[selectedProvider.value] || []);

watch(defaultProviderId, (providerId) => {
  if (!selectedProvider.value && providerId) selectedProvider.value = props.providerId || providerId;
}, { immediate: true });

watch(() => props.providerId, (providerId) => {
  if (providerId && providerId !== selectedProvider.value) selectedProvider.value = providerId;
});

watch(() => props.model, (model) => {
  selectedModel.value = model || providerDefaultModel.value;
});

watch([selectedProvider, selectedModel], () => {
  emit("change", { provider_id: selectedProvider.value, model: selectedModel.value });
}, { immediate: true });

watch(selectedProvider, () => {
  selectedModel.value = props.model || providerDefaultModel.value;
  void loadModels(false);
}, { immediate: true });

async function loadModels(force: boolean) {
  if (!selectedProvider.value) return;
  if (appState.models[selectedProvider.value] && !force) {
    ensureSelectedModel();
    return;
  }
  loading.value = true;
  status.value = "Loading models...";
  try {
    const payload = await api<{ models: Model[] }>(`/api/providers/${encodeURIComponent(selectedProvider.value)}/models`);
    appState.models[selectedProvider.value] = payload.models || [];
    ensureSelectedModel();
    status.value = appState.models[selectedProvider.value].length ? "Models loaded" : "No models returned";
  } catch (error) {
    status.value = error instanceof Error ? error.message : "Model load failed";
    appState.models[selectedProvider.value] = [];
    selectedModel.value = providerDefaultModel.value;
  } finally {
    loading.value = false;
  }
}

function ensureSelectedModel() {
  if (models.value.some((model) => model.id === selectedModel.value)) return;
  const fallback = providerDefaultModel.value;
  if (!models.value.length) {
    selectedModel.value = fallback;
    return;
  }
  selectedModel.value = models.value.find((model) => model.id === fallback)?.id || models.value[0]?.id || fallback;
}
</script>

<template>
  <div class="field-row">
    <div class="field">
      <label>Provider</label>
      <select v-model="selectedProvider">
        <option v-if="!providers.length" value="">No providers configured</option>
        <option v-for="provider in providers" :key="provider.id" :value="provider.id">
          {{ provider.name || provider.id }}
        </option>
      </select>
    </div>
    <div class="field">
      <label>Model</label>
      <select v-model="selectedModel">
        <option v-if="loading" value="">Loading models...</option>
        <option v-else-if="!models.length" :value="providerDefaultModel">{{ providerDefaultModel || status || "Load models..." }}</option>
        <option v-for="modelOption in models" v-else :key="modelOption.id" :value="modelOption.id">
          {{ modelOption.name || modelOption.id }}
        </option>
      </select>
    </div>
    <div v-if="includeRefresh" class="field">
      <button type="button" :disabled="loading" @click="loadModels(true)">Refresh models</button>
    </div>
  </div>
</template>
