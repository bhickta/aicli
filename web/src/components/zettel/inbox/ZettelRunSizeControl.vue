<script setup lang="ts">
import { computed } from "vue";

const model = defineModel<number>({ required: true });

withDefaults(defineProps<{
  disabled?: boolean;
  label?: string;
  ariaLabel?: string;
  emptyText?: string;
  activeText?: string;
}>(), {
  label: "Run size",
  ariaLabel: "Inbox merge run size",
  emptyText: "Full inbox",
  activeText: "First",
});

const presetLimits = [5, 10, 25, 50];
const normalizedLimit = computed(() => {
  const value = Math.floor(Number(model.value));
  return Number.isFinite(value) && value > 0 ? value : 0;
});
const customLimit = computed({
  get: () => normalizedLimit.value ? String(normalizedLimit.value) : "",
  set: (value: string) => {
    const next = Math.floor(Number(value));
    model.value = Number.isFinite(next) && next > 0 ? next : 0;
  },
});

function selectAll() {
  model.value = 0;
}

function selectLimit(value: number) {
  model.value = value;
}
</script>

<template>
  <div class="field run-size-control">
    <label for="zettel-run-limit">{{ label }}</label>
    <div class="run-size-toolbar" role="group" :aria-label="ariaLabel">
      <button
        type="button"
        class="run-size-option"
        :class="{ active: normalizedLimit === 0 }"
        :disabled="disabled"
        @click="selectAll"
      >
        All
      </button>
      <button
        v-for="limit in presetLimits"
        :key="limit"
        type="button"
        class="run-size-option"
        :class="{ active: normalizedLimit === limit }"
        :disabled="disabled"
        @click="selectLimit(limit)"
      >
        {{ limit }}
      </button>
      <input
        id="zettel-run-limit"
        v-model="customLimit"
        type="number"
        min="1"
        step="1"
        inputmode="numeric"
        placeholder="N"
        :disabled="disabled"
        aria-label="Custom note limit"
      >
    </div>
    <small>{{ normalizedLimit === 0 ? emptyText : `${activeText} ${normalizedLimit} notes` }}</small>
  </div>
</template>

<style scoped>
.run-size-control {
  max-width: 520px;
}

.run-size-toolbar {
  display: grid;
  grid-template-columns: repeat(5, minmax(44px, auto)) minmax(88px, 1fr);
  gap: 8px;
  align-items: center;
}

.run-size-option {
  min-height: 38px;
  text-align: center;
}

.run-size-option.active {
  border-color: #6ea8fe;
  background: #263957;
}

.run-size-toolbar input {
  min-width: 0;
}

.run-size-control small {
  color: #9aa4b2;
}

@media (max-width: 620px) {
  .run-size-toolbar {
    grid-template-columns: repeat(3, minmax(44px, 1fr));
  }
}
</style>
