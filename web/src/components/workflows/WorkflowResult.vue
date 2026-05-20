<script setup lang="ts">
import { computed } from "vue";
import { progressBarWidth } from "../../lib/jobProgress";
import type { ProgressMode } from "../../types";

const props = defineProps<{
  status: string;
  progress: number;
  progressMode: ProgressMode;
  progressVisible: boolean;
  result: string;
  sourcePreview: string;
  markdownPreview: string;
}>();

const progressClass = computed(() => ({
  hidden: !props.progressVisible,
  indeterminate: props.progressMode === "indeterminate",
}));
const progressStyle = computed(() => ({
  width: progressBarWidth(props.progressMode, props.progress),
}));
</script>

<template>
  <div class="field">
    <h3>Status</h3>
    <p id="workflow-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
  </div>
  <div id="workflow-progress" class="progress" :class="progressClass">
    <div :style="progressStyle" />
  </div>
  <div class="field">
    <h3>Result</h3>
    <pre id="workflow-result" role="status" aria-live="polite">{{ result }}</pre>
  </div>
  <div id="review-pane" class="review-pane" :class="{ hidden: !sourcePreview && !markdownPreview }">
    <iframe id="source-preview" title="Source file preview" :src="sourcePreview || undefined" />
    <textarea id="markdown-preview" :value="markdownPreview" readonly placeholder="Markdown preview appears here" />
  </div>
</template>
