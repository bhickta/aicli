<script setup lang="ts">
import WhatsAppContactField from "./WhatsAppContactField.vue";
import type { WorkflowField } from "../../types";

const props = defineProps<{
  field: WorkflowField;
  value: unknown;
}>();

const emit = defineEmits<{
  update: [id: string, value: unknown];
  browse: [field: WorkflowField];
}>();

function update(value: unknown) {
  if (!props.field.id) return;
  emit("update", props.field.id, value);
}

function forwardUpdate(id: string, value: unknown) {
  emit("update", id, value);
}

function datetimeLocalValue(value: unknown) {
  const text = String(value ?? props.field.value ?? "");
  const match = text.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2})(?::\d{2})?(?:[+-]\d{2}:\d{2}|Z)?$/);
  return match?.[1] || text;
}
</script>

<template>
  <div v-if="field.id && field.type === 'text'" class="field">
    <label :for="`wf-${field.id}`">{{ field.label }}</label>
    <input :id="`wf-${field.id}`" type="text" :value="String(value ?? field.value ?? '')" :placeholder="field.placeholder || ''" @input="update(($event.target as HTMLInputElement).value)">
  </div>

  <div v-else-if="field.id && field.type === 'datetime'" class="field">
    <label :for="`wf-${field.id}`">{{ field.label }}</label>
    <input :id="`wf-${field.id}`" type="datetime-local" :value="datetimeLocalValue(value)" :placeholder="field.placeholder || ''" @input="update(($event.target as HTMLInputElement).value)">
  </div>

  <div v-else-if="field.id && field.type === 'textarea'" class="field">
    <label :for="`wf-${field.id}`">{{ field.label }}</label>
    <textarea :id="`wf-${field.id}`" :rows="field.rows || 6" :placeholder="field.placeholder || ''" :value="String(value ?? '')" @input="update(($event.target as HTMLTextAreaElement).value)" />
  </div>

  <div v-else-if="field.id && field.type === 'select'" class="field">
    <label :for="`wf-${field.id}`">{{ field.label }}</label>
    <select :id="`wf-${field.id}`" :value="String(value ?? field.default ?? '')" @change="update(($event.target as HTMLSelectElement).value)">
      <option v-for="option in field.options || []" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  </div>

  <div v-else-if="field.id && field.type === 'number'" class="field">
    <label :for="`wf-${field.id}`">{{ field.label }}</label>
    <input :id="`wf-${field.id}`" type="number" :min="field.min" :max="field.max" :value="Number(value ?? field.default ?? 0)" @input="update(Number(($event.target as HTMLInputElement).value))">
  </div>

  <div v-else-if="field.id && field.type === 'checkbox'" class="field">
    <label class="checkbox">
      <input type="checkbox" :checked="Boolean(value ?? field.checked)" @change="update(($event.target as HTMLInputElement).checked)">
      <span>{{ field.label }}</span>
    </label>
  </div>

  <div v-else-if="field.id && field.type === 'path'" class="field">
    <label>{{ field.label }}</label>
    <div class="path-control">
      <output :data-path="String(value || '')">{{ value || `No ${(field.label || 'path').toLowerCase()} selected` }}</output>
      <button type="button" @click="emit('browse', field)">
        {{ field.picker === "directory" ? `Browse ${field.label}` : `Choose ${field.label}` }}
      </button>
    </div>
  </div>

  <WhatsAppContactField
    v-else-if="field.id && field.type === 'whatsappContact'"
    :field="field"
    :value="value"
    @update="forwardUpdate"
  />
</template>
