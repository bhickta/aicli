<script setup lang="ts">
import { computed } from "vue";
import ProviderModelControl from "../../ProviderModelControl.vue";
import { providers } from "../../../stores/appState";

interface ProviderSettings {
  mergeProviderId: string;
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

function updateProviderModel(
  value: { provider_id: string; model: string },
) {
  emit("update", {
    mergeProviderId: value.provider_id,
    mergeModel: value.model,
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
      <h3>AI merge model</h3>
      <ProviderModelControl
        :provider-id="settings.mergeProviderId"
        :model="settings.mergeModel"
        @change="updateProviderModel"
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
