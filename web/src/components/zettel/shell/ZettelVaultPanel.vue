<script setup lang="ts">
import ZettelSection from "../common/ZettelSection.vue";

defineProps<{
  vaultPath: string;
  busy: boolean;
}>();

const emit = defineEmits<{
  pick: [];
}>();
</script>

<template>
  <ZettelSection title="Vault" :description="vaultPath || 'No vault selected'">
    <template #actions>
      <button type="button" :disabled="busy" @click="emit('pick')">Browse vault</button>
    </template>
  </ZettelSection>
</template>
