<script setup lang="ts">
import ProviderModelControl from "../ProviderModelControl.vue";
import type { ProviderModelSelection } from "../../composables/useWorkflowForm";
import type { WorkflowField as WorkflowFieldType } from "../../types";
import WorkflowField from "./WorkflowField.vue";

defineProps<{
  fields: WorkflowFieldType[];
  values: Record<string, unknown>;
  hasProviderModel: boolean;
  providerModel: ProviderModelSelection;
  preferredProviderId?: string;
  preferredModel?: string;
}>();

const emit = defineEmits<{
  browse: [field: WorkflowFieldType];
  updateField: [id: string, value: unknown];
  updateProviderModel: [value: ProviderModelSelection];
}>();
</script>

<template>
  <div id="workflow-fields" class="grid">
    <ProviderModelControl
      v-if="hasProviderModel"
      :provider-id="providerModel.provider_id || preferredProviderId || ''"
      :model="providerModel.model || preferredModel || ''"
      @change="emit('updateProviderModel', $event)"
    />
    <WorkflowField
      v-for="field in fields"
      :key="field.id"
      :field="field"
      :value="field.id ? values[field.id] : ''"
      @update="(id, value) => emit('updateField', id, value)"
      @browse="emit('browse', $event)"
    />
  </div>
</template>
