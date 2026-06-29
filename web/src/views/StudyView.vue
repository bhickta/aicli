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
    <PageHeader title="Study" description="Analyze UPSC answer copies, review saved OCR, and run study utilities.">
      <template #actions>
        <PageTabs :tabs="tabs" label="Study sections" />
      </template>
    </PageHeader>

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

.study-content {
  align-self: start;
  min-width: 0;
}
</style>
