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

function updateEmbeddingProviderModel(value: { provider_id: string; model: string }) {
  emit("update", {
    embeddingProviderId: value.provider_id,
    embeddingModel: value.model,
  });
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
      <ProviderModelControl
        :provider-id="settings.embeddingProviderId"
        :model="settings.embeddingModel"
        :provider-options="embeddingProviders"
        @change="updateEmbeddingProviderModel"
      />
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
