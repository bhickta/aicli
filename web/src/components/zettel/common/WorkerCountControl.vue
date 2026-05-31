<script setup lang="ts">
import { computed } from "vue";

const model = defineModel<number>({ required: true });

const props = withDefaults(defineProps<{
  id: string;
  label: string;
  options: number[];
  disabled?: boolean;
  min?: number;
  unit?: string;
  helper?: string;
}>(), {
  min: 1,
  unit: "at once",
  helper: "",
});

const normalizedValue = computed(() => normalizeWorkerCount(model.value));
const customValue = computed({
  get: () => String(normalizedValue.value),
  set: (value: string) => {
    model.value = normalizeWorkerCount(value);
  },
});

function selectWorkerCount(value: number) {
  model.value = normalizeWorkerCount(value);
}

function normalizeWorkerCount(value: unknown) {
  const parsed = Math.floor(Number(value));
  if (!Number.isFinite(parsed)) return props.min;
  return Math.max(props.min, parsed);
}
</script>

<template>
  <div class="field worker-count-control">
    <label :for="`${id}-custom`">{{ label }}</label>
    <div class="worker-count-toolbar" role="group" :aria-label="label">
      <button
        v-for="option in options"
        :key="option"
        type="button"
        class="worker-count-option"
        :class="{ active: normalizedValue === option }"
        :disabled="disabled"
        @click="selectWorkerCount(option)"
      >
        {{ option }}
      </button>
      <input
        :id="`${id}-custom`"
        v-model="customValue"
        type="number"
        :min="min"
        step="1"
        inputmode="numeric"
        :disabled="disabled"
        aria-label="Custom worker count"
      >
    </div>
    <small>{{ helper || `${normalizedValue} ${unit}` }}</small>
  </div>
</template>

<style scoped>
.worker-count-control {
  min-width: 0;
}

.worker-count-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

.worker-count-option {
  min-height: 38px;
  padding-inline: 12px;
  text-align: center;
}

.worker-count-option.active {
  border-color: #6ea8fe;
  background: #263957;
}

.worker-count-toolbar input {
  flex: 1 1 82px;
  min-width: 0;
  max-width: 140px;
}

.worker-count-control small {
  color: #9aa4b2;
}

</style>
