<script setup lang="ts">
defineProps<{
  status: string;
  progress: number;
  result: string;
  sourcePreview: string;
  markdownPreview: string;
}>();
</script>

<template>
  <div class="field">
    <h3>Status</h3>
    <p id="workflow-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
  </div>
  <div id="workflow-progress" class="progress" :class="{ hidden: progress <= 0 }">
    <div :style="{ width: `${Math.max(0, Math.min(100, progress))}%` }" />
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
