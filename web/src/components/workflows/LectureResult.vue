<script setup lang="ts">
import type { LectureResult } from "../../types";

defineProps<{
  result: LectureResult;
}>();
</script>

<template>
  <section class="lecture-result">
    <header class="lecture-result-header">
      <div>
        <h3>{{ result.title }}</h3>
        <p>{{ result.source_notes.length }} source note(s) · {{ result.input_chars }} input chars · {{ result.skipped_notes }} skipped</p>
      </div>
      <div class="lecture-result-actions">
        <a v-if="result.script_url" :href="result.script_url" target="_blank" rel="noreferrer">Open script</a>
        <a v-if="result.audio_url" :href="result.audio_url" target="_blank" rel="noreferrer">Open audio</a>
      </div>
    </header>

    <div v-if="result.audio_url" class="lecture-audio">
      <audio controls :src="result.audio_url" />
    </div>

    <details class="lecture-sources">
      <summary>Source notes</summary>
      <ul>
        <li v-for="note in result.source_notes" :key="note">{{ note }}</li>
      </ul>
    </details>

    <article class="lecture-script">
      <pre>{{ result.script }}</pre>
    </article>
  </section>
</template>

<style scoped>
.lecture-result {
  background: #0d121b;
  border: 1px solid #253247;
  border-radius: 7px;
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 10px;
}

.lecture-result-header,
.lecture-result-actions {
  align-items: center;
  display: flex;
  gap: 10px;
  justify-content: space-between;
  min-width: 0;
}

.lecture-result-header h3,
.lecture-result-header p {
  margin: 0;
}

.lecture-result-header h3 {
  font-size: 16px;
}

.lecture-result-header p,
.lecture-sources {
  color: #94a3b8;
  font-size: 12px;
}

.lecture-result-actions {
  flex-wrap: wrap;
  justify-content: flex-end;
}

.lecture-result-actions a {
  background: #1e3a5f;
  border: 1px solid #60a5fa;
  border-radius: 4px;
  color: #dbeafe;
  padding: 6px 10px;
  text-decoration: none;
}

.lecture-audio audio {
  width: 100%;
}

.lecture-sources ul {
  margin: 8px 0 0;
  max-height: 10rem;
  overflow: auto;
  padding-left: 18px;
}

.lecture-script {
  background: #020617;
  border: 1px solid #253247;
  border-radius: 5px;
  overflow: auto;
  padding: 10px;
}

.lecture-script pre {
  color: #e5e7eb;
  font: 0.92rem/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  margin: 0;
  white-space: pre-wrap;
}

@media (max-width: 760px) {
  .lecture-result-header,
  .lecture-result-actions {
    align-items: stretch;
    display: grid;
    justify-content: stretch;
  }
}
</style>
