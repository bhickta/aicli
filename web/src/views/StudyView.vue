<script setup lang="ts">
import { computed, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "../components/layout/PageHeader.vue";
import PageTabs from "../components/layout/PageTabs.vue";
import StudyAnalyticsPanel from "../components/study/StudyAnalyticsPanel.vue";
import StudyCopyTable from "../components/study/StudyCopyTable.vue";
import StudyImportPanel from "../components/study/StudyImportPanel.vue";
import StudyQuestionsPanel from "../components/study/StudyQuestionsPanel.vue";
import StudyWorkflowPanel from "../components/study/StudyWorkflowPanel.vue";
import { useStudyCopies } from "../composables/useStudyCopies";
import StudyArchiveView from "./StudyArchiveView.vue";

type StudySection = "questions" | "analytics" | "archive" | "import" | "run";

const route = useRoute();
const router = useRouter();
const study = useStudyCopies();
const {
  status,
  query,
  copies,
  selected,
  selectedIds,
  importPath,
  importFolder,
  summary,
  loadCopies,
  openCopy: loadCopy,
  importCopies,
  startBatch,
  toggleCopy,
} = study;

const activeSection = computed<StudySection>(() => {
  const section = String(route.params.section || "questions");
  if (["analytics", "archive", "import", "run"].includes(section)) return section as StudySection;
  return "questions";
});

const tabs = computed(() => [
  { label: "Questions", to: { name: "study", params: { section: "questions" } }, active: activeSection.value === "questions" },
  { label: "Analytics", to: { name: "study", params: { section: "analytics" } }, active: activeSection.value === "analytics" },
  { label: "Archive", to: { name: "study", params: { section: "archive" } }, active: activeSection.value === "archive" },
  { label: "Import", to: { name: "study", params: { section: "import" } }, active: activeSection.value === "import" },
  { label: "Run", to: { name: "study", params: { section: "run" } }, active: activeSection.value === "run" },
]);

const activeCopyId = computed(() => selected.value?.copy.id || "");

onMounted(async () => {
  await loadCopies();
  const id = String(route.params.copyId || "");
  if (id) await loadCopy(id);
});

watch(
  () => route.params.copyId,
  async (id) => {
    if (typeof id === "string" && id && id !== activeCopyId.value) await study.openCopy(id);
  },
);

async function openCopy(id: string) {
  await router.push({ name: "study", params: { section: activeSection.value, copyId: id } });
  await loadCopy(id);
}
</script>

<template>
  <div class="study-shell">
    <header class="study-topbar">
      <PageHeader title="Study" description="Track answer copies, split questions, and review analytics." />
      <PageTabs :tabs="tabs" label="Study sections" />
    </header>

    <main v-if="activeSection === 'run'" class="study-run">
      <StudyWorkflowPanel />
    </main>

    <main v-else-if="activeSection === 'archive'" class="study-run">
      <StudyArchiveView />
    </main>

    <main v-else class="study-workspace">
      <aside class="study-sidebar">
        <div class="study-toolbar">
          <input v-model="query" type="search" placeholder="Search copies, candidate, paper" @keyup.enter="loadCopies" />
          <button type="button" @click="loadCopies">Search</button>
        </div>
        <p class="study-summary">{{ summary }} · {{ status }}</p>
        <div class="study-batchbar">
          <button type="button" :disabled="!selectedIds.length" @click="startBatch('all')">Queue selected</button>
          <button type="button" :disabled="!selectedIds.length" @click="startBatch('ocr')">OCR</button>
        </div>
        <StudyCopyTable
          :copies="copies"
          :active-id="activeCopyId"
          :selected-ids="selectedIds"
          @open="openCopy"
          @toggle="toggleCopy"
        />
      </aside>

      <section class="study-main">
        <StudyImportPanel
          v-if="activeSection === 'import'"
          v-model:import-path="importPath"
          v-model:import-folder="importFolder"
          @import="importCopies"
        />
        <StudyAnalyticsPanel v-else-if="activeSection === 'analytics'" :detail="selected" />
        <StudyQuestionsPanel v-else :detail="selected" />
      </section>
    </main>
  </div>
</template>
