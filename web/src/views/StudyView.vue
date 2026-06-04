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
      <div>
        <h2>Study</h2>
        <p class="muted">Study workflows and saved answer-copy reviews.</p>
      </div>
    </header>

    <nav class="study-subtabs" aria-label="Study sections">
      <button
        type="button"
        :class="{ active: activeSection === 'topper-copies' }"
        @click="activeSection = 'topper-copies'"
      >
        Topper answer copies
      </button>
      <button
        type="button"
        :class="{ active: activeSection === 'workflows' }"
        @click="activeSection = 'workflows'"
      >
        Workflows
      </button>
    </nav>

    <StudyArchiveView v-if="activeSection === 'topper-copies'" />
    <StudyWorkflowPanel v-else />
  </div>
</template>

<style scoped>
.study-panel {
  display: grid;
  gap: 14px;
  min-height: calc(100vh - 7.5rem);
}

.study-header h2,
.study-header p {
  margin: 0;
}

.study-subtabs {
  border-bottom: 1px solid #2b3440;
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding-bottom: 8px;
}

.study-subtabs button.active {
  border-color: #69a1ff;
  background: #17304f;
}
</style>
