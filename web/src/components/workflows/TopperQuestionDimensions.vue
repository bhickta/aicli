<script setup lang="ts">
import { computed } from "vue";
import type { QuestionDimensions } from "../../types";

const props = defineProps<{
  dimensions?: QuestionDimensions;
}>();

const rows = computed(() => {
  const dimensions = props.dimensions;
  if (!dimensions) return [];
  return [
    ["Introduction", dimensions.introduction],
    ["Conclusion", dimensions.outro],
    ["Transitions", dimensions.transition],
    ["Diagram", dimensions.diagram],
    ["Facts", dimensions.fact],
    ["Fact usage", dimensions.fact_usage],
    ["Custom", dimensions.custom],
  ].filter(([, value]) => String(value || "").trim() !== "");
});
</script>

<template>
  <section v-if="rows.length" class="topper-dimensions">
    <h5>Question dimensions</h5>
    <dl>
      <div v-for="[label, value] in rows" :key="label">
        <dt>{{ label }}</dt>
        <dd>{{ value }}</dd>
      </div>
    </dl>
  </section>
</template>
