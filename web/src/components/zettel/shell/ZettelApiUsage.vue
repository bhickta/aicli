<script setup lang="ts">
import { computed } from "vue";
import type { ApiCallUsage, ProviderApiCallUsage } from "../../../types";
import ZettelBadges from "../common/ZettelBadges.vue";

const props = defineProps<{
  usage: ApiCallUsage | null;
}>();

const badges = computed(() => {
  if (!props.usage) return [];
  return [
    `${props.usage.total} total`,
    `${props.usage.chat} chat`,
    `${props.usage.embeddings} embeddings`,
    `${props.usage.vision} vision`,
    `${props.usage.stream} stream`,
  ].filter((item) => !item.startsWith("0 ") || item === "0 total");
});

const providerRows = computed(() => (props.usage?.providers || [])
  .filter((provider) => provider.total > 0)
  .map(formatProviderUsage));

function formatProviderUsage(provider: ProviderApiCallUsage) {
  const parts = [
    `${provider.total} total`,
    provider.chat ? `${provider.chat} chat` : "",
    provider.embeddings ? `${provider.embeddings} embeddings` : "",
    provider.vision ? `${provider.vision} vision` : "",
    provider.stream ? `${provider.stream} stream` : "",
  ].filter(Boolean);
  return `${provider.provider_id}: ${parts.join(", ")}`;
}
</script>

<template>
  <aside v-if="usage" class="api-usage" aria-label="Provider API calls">
    <div class="api-usage-header">
      <strong>Provider API calls</strong>
      <ZettelBadges :items="badges" />
    </div>
    <ul v-if="providerRows.length" class="api-usage-providers">
      <li v-for="row in providerRows" :key="row">{{ row }}</li>
    </ul>
  </aside>
</template>

<style scoped>
.api-usage {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #10141b;
}

.api-usage-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.api-usage-providers {
  display: grid;
  gap: 4px;
  margin: 0;
  padding-left: 18px;
  color: #9aa4b2;
}

@media (max-width: 760px) {
  .api-usage-header {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
