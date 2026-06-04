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
      </div>
      <nav class="study-tabs" aria-label="Study sections">
        <button
          type="button"
          :class="{ active: activeSection === 'topper-copies' }"
          @click="activeSection = 'topper-copies'"
        >
          Topper copies
        </button>
        <button
          type="button"
          :class="{ active: activeSection === 'workflows' }"
          @click="activeSection = 'workflows'"
        >
          Workflows
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
  gap: 12px;
  min-height: calc(100vh - 7.5rem);
  padding: 14px;
}

.study-header {
  align-items: center;
  border-bottom: 1px solid #2b3440;
  display: flex;
  gap: 16px;
  justify-content: space-between;
  padding-bottom: 10px;
}

.study-header h2 {
  margin: 0;
  font-size: 20px;
}

.study-tabs {
  background: #0f141c;
  border: 1px solid #2b3440;
  border-radius: 7px;
  display: flex;
  gap: 2px;
  padding: 3px;
}

.study-tabs button {
  border: 0;
  border-radius: 5px;
  background: transparent;
  min-width: 8rem;
  padding: 7px 12px;
  text-align: center;
}

.study-tabs button.active {
  background: #17304f;
  color: #ffffff;
}

.study-content {
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
