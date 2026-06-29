<script setup lang="ts">
import { shallowRef } from "vue";
import StudyWorkflowPanel from "../components/study/StudyWorkflowPanel.vue";
import StudyArchiveView from "./StudyArchiveView.vue";

type StudySection = "workflows" | "topper-copies";

const activeSection = shallowRef<StudySection>("topper-copies");
</script>

<template>
  <div class="study-panel panel">
    <header class="study-header">
      <div class="study-title">
        <h2>Study</h2>
        <p>Analyze UPSC answer copies, review saved OCR, and run study utilities.</p>
      </div>
      <nav class="study-tabs" aria-label="Study sections">
        <button
          type="button"
          :class="{ active: activeSection === 'topper-copies' }"
          @click="activeSection = 'topper-copies'"
        >
          Saved copies
        </button>
        <button
          type="button"
          :class="{ active: activeSection === 'workflows' }"
          @click="activeSection = 'workflows'"
        >
          Run analysis
        </button>
      </nav>
    </header>

    <main class="study-content">
      <StudyArchiveView v-if="activeSection === 'topper-copies'" />
      <StudyWorkflowPanel v-else />
    </main>
  </div>
</template>

<style scoped>
.study-panel {
  display: grid;
  align-content: start;
  gap: 10px;
  min-height: calc(100vh - 7.5rem);
  padding: 12px;
}

.study-header {
  align-items: center;
  background: #0f141c;
  border: 1px solid #2b3440;
  border-radius: 7px;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  min-width: 0;
  padding: 8px;
}

.study-title {
  min-width: 0;
}

.study-header h2 {
  margin: 0;
  font-size: 18px;
  line-height: 1.2;
}

.study-title p {
  color: #94a3b8;
  font-size: 13px;
  margin: 3px 0 0;
}

.study-tabs {
  background: #0a0f18;
  border: 1px solid #2b3440;
  border-radius: 6px;
  display: flex;
  gap: 2px;
  padding: 3px;
}

.study-tabs button {
  border: 0;
  border-radius: 4px;
  background: transparent;
  min-height: 2rem;
  min-width: 7.5rem;
  padding: 5px 10px;
  text-align: center;
}

.study-tabs button.active {
  background: #17304f;
  color: #ffffff;
}

.study-content {
  align-self: start;
  min-width: 0;
}

@media (max-width: 760px) {
  .study-header {
    align-items: stretch;
    display: grid;
  }

  .study-tabs {
    overflow-x: auto;
  }

  .study-tabs button {
    min-width: 7rem;
    white-space: nowrap;
  }
}
</style>
