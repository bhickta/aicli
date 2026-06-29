<script setup lang="ts">
import { computed } from "vue";
import { useRoute } from "vue-router";
import PageHeader from "../components/layout/PageHeader.vue";
import PageTabs from "../components/layout/PageTabs.vue";
import StudyWorkflowPanel from "../components/study/StudyWorkflowPanel.vue";
import StudyArchiveView from "./StudyArchiveView.vue";

type StudySection = "workflows" | "topper-copies";

const route = useRoute();
const activeSection = computed<StudySection>(() => {
  return route.params.section === "run" ? "workflows" : "topper-copies";
});
const tabs = computed(() => [
  { label: "Saved copies", to: { name: "study", params: { section: "copies" } }, active: activeSection.value === "topper-copies" },
  { label: "Run analysis", to: { name: "study", params: { section: "run" } }, active: activeSection.value === "workflows" },
]);
</script>

<template>
  <div class="study-panel panel">
    <div class="study-page-header">
      <PageHeader title="Study" description="Analyze UPSC answer copies, review saved OCR, and run study utilities." />
      <PageTabs :tabs="tabs" label="Study sections" />
    </div>

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
  gap: 12px;
  min-height: calc(100vh - 10rem);
  min-width: 0;
  padding: 12px;
}

.study-page-header {
  align-items: start;
  background: #0f141c;
  border: 1px solid #2b3440;
  border-radius: 7px;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(0, 1fr) auto;
  min-width: 0;
  padding: 8px;
}

.study-content {
  align-self: start;
  min-width: 0;
}

@media (max-width: 900px) {
  .study-page-header {
    grid-template-columns: 1fr;
  }
}
</style>
