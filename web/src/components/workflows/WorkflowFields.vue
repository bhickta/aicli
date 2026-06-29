<script setup lang="ts">
import ProviderModelControl from "../ProviderModelControl.vue";
import type { ProviderModelSelection } from "../../composables/useWorkflowForm";
import type { WorkflowField as WorkflowFieldType } from "../../types";
import WorkflowField from "./WorkflowField.vue";

const props = defineProps<{
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

function stepProviderID(field: WorkflowFieldType) {
  if (!field.id) return "";
  return String(propsValue(field.id + "_provider_id"));
}

function stepModel(field: WorkflowFieldType) {
  if (!field.id) return "";
  return String(propsValue(field.id + "_model"));
}

function propsValue(id: string) {
  return props.values[id] ?? "";
}

function updateStepProviderModel(field: WorkflowFieldType, value: ProviderModelSelection) {
  if (!field.id) return;
  emit("updateField", field.id + "_provider_id", value.provider_id);
  emit("updateField", field.id + "_model", value.model);
}
</script>

<template>
  <div id="workflow-fields" class="workflow-fields grid">
    <ProviderModelControl
      v-if="hasProviderModel"
      :provider-id="providerModel.provider_id || preferredProviderId || ''"
      :model="providerModel.model || preferredModel || ''"
      @change="emit('updateProviderModel', $event)"
    />
    <template v-for="field in fields" :key="field.id">
      <div v-if="field.id && field.type === 'stepProviderModel'" class="field">
        <label>{{ field.label }}</label>
        <ProviderModelControl
          :provider-id="stepProviderID(field)"
          :model="stepModel(field)"
          @change="updateStepProviderModel(field, $event)"
        />
      </div>
      <WorkflowField
        v-else
        :field="field"
        :value="field.id ? values[field.id] : ''"
        @update="(id, value) => emit('updateField', id, value)"
        @browse="emit('browse', $event)"
      />
    </template>
  </div>
</template>
