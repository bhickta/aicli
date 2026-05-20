<script setup lang="ts">
import { computed } from "vue";
import ProviderModelControl from "../ProviderModelControl.vue";
import { providers } from "../../stores/appState";

interface ProviderSettings {
  candidateProviderId: string;
  mergeProviderId: string;
  validationProviderId: string;
  candidateModel: string;
  mergeModel: string;
  validationModel: string;
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

function updateProviderModel(
  providerKey: "candidateProviderId" | "mergeProviderId" | "validationProviderId",
  modelKey: "candidateModel" | "mergeModel" | "validationModel",
  value: { provider_id: string; model: string },
) {
  emit("update", {
    [providerKey]: value.provider_id,
    [modelKey]: value.model,
  });
}

function updateField(key: keyof ProviderSettings, value: string) {
  emit("update", { [key]: value });
}
</script>

<template>
  <div class="zettel-provider-settings">
    <section class="provider-section">
      <h3>Candidate judge</h3>
      <ProviderModelControl
        :provider-id="settings.candidateProviderId"
        :model="settings.candidateModel"
        @change="updateProviderModel('candidateProviderId', 'candidateModel', $event)"
      />
    </section>

    <section class="provider-section">
      <h3>Merge planner</h3>
      <ProviderModelControl
        :provider-id="settings.mergeProviderId"
        :model="settings.mergeModel"
        @change="updateProviderModel('mergeProviderId', 'mergeModel', $event)"
      />
    </section>

    <section class="provider-section">
      <h3>Validation judge</h3>
      <ProviderModelControl
        :provider-id="settings.validationProviderId"
        :model="settings.validationModel"
        @change="updateProviderModel('validationProviderId', 'validationModel', $event)"
      />
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
