<script setup lang="ts">
import { computed } from "vue";
import ProviderModelControl from "../ProviderModelControl.vue";
import { providers } from "../../stores/appState";

interface ProviderSettings {
  providerId: string;
  judgeModel: string;
  mergeModel: string;
  embeddingProviderId: string;
  embeddingModel: string;
}

const props = defineProps<{
  settings: ProviderSettings;
}>();

const emit = defineEmits<{
  update: [value: Partial<ProviderSettings>];
}>();

const embeddingProviders = computed(() => providers.value.filter((provider) => provider.type !== "codex-cli"));

function updateLLMProvider(value: { provider_id: string; model: string }) {
  const patch: Partial<ProviderSettings> = { providerId: value.provider_id };
  if (value.model) {
    patch.judgeModel = value.model;
    patch.mergeModel = value.model;
  }
  emit("update", patch);
}

function updateField(key: keyof ProviderSettings, value: string) {
  emit("update", { [key]: value });
}
</script>

<template>
  <div class="zettel-provider-settings">
    <section class="provider-section">
      <h3>Merge provider</h3>
      <ProviderModelControl :provider-id="settings.providerId" :model="settings.judgeModel" @change="updateLLMProvider" />
      <div class="field-row">
        <div class="field">
          <label for="zettel-judge">Judge model</label>
          <input id="zettel-judge" :value="settings.judgeModel" type="text" @input="updateField('judgeModel', ($event.target as HTMLInputElement).value)">
        </div>
        <div class="field">
          <label for="zettel-merge">Merge model</label>
          <input id="zettel-merge" :value="settings.mergeModel" type="text" @input="updateField('mergeModel', ($event.target as HTMLInputElement).value)">
        </div>
      </div>
    </section>

    <section class="provider-section">
      <h3>Embedding provider</h3>
      <div class="field-row">
        <div class="field">
          <label for="zettel-embed-provider">Provider</label>
          <select id="zettel-embed-provider" :value="settings.embeddingProviderId" @change="updateField('embeddingProviderId', ($event.target as HTMLSelectElement).value)">
            <option v-for="provider in embeddingProviders" :key="provider.id" :value="provider.id">
              {{ provider.name || provider.id }}
            </option>
          </select>
        </div>
        <div class="field">
          <label for="zettel-embed">Embedding model</label>
          <input id="zettel-embed" :value="settings.embeddingModel" type="text" @input="updateField('embeddingModel', ($event.target as HTMLInputElement).value)">
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.zettel-provider-settings {
  display: grid;
  gap: 14px;
}

.provider-section {
  display: grid;
  gap: 10px;
}
</style>
