<script setup lang="ts">
import { computed } from "vue";
import { progressBarWidth } from "../../lib/jobProgress";
import type { ProgressMode, TopperCopyReview } from "../../types";
import TopperCopyReviewResult from "./TopperCopyReviewResult.vue";

const props = defineProps<{
  status: string;
  progress: number;
  progressMode: ProgressMode;
  progressVisible: boolean;
  result: string;
  parsedResult: unknown;
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
const topperCopyReview = computed(() => {
  if (props.parsedResult && typeof props.parsedResult === "object" && (props.parsedResult as { kind?: unknown }).kind === "topper_copy_review") {
    return props.parsedResult as TopperCopyReview;
  }
  return null;
});
</script>

<template>
  <div class="field">
    <h3>Status</h3>
    <p id="workflow-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
  </div>
  <div id="workflow-progress" class="progress" :class="progressClass">
    <div :style="progressStyle" />
  </div>
  <TopperCopyReviewResult v-if="topperCopyReview" :review="topperCopyReview" />
  <div v-else class="field">
    <h3>Result</h3>
    <pre id="workflow-result" role="status" aria-live="polite">{{ result }}</pre>
  </div>
  <div v-if="!topperCopyReview" id="review-pane" class="review-pane" :class="{ hidden: !sourcePreview && !markdownPreview }">
    <iframe id="source-preview" title="Source file preview" :src="sourcePreview || undefined" />
    <textarea id="markdown-preview" :value="markdownPreview" readonly placeholder="Markdown preview appears here" />
  </div>
</template>
