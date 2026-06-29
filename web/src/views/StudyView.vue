<script setup lang="ts">
import { computed, onMounted, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "../components/layout/PageHeader.vue";
import StudyCopyTable from "../components/study/StudyCopyTable.vue";
import StudyQuestionsPanel from "../components/study/StudyQuestionsPanel.vue";
import StudyWorkflowPanel from "../components/study/StudyWorkflowPanel.vue";
import { useStudyCopies } from "../composables/useStudyCopies";

const route = useRoute();
const router = useRouter();
const study = useStudyCopies();
const {
  status,
  query,
  copies,
  selected,
  selectedIds,
  summary,
  loadCopies,
  openCopy: loadCopy,
  startBatch,
  toggleCopy,
} = study;

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
  await router.push({ name: "study", params: { copyId: id } });
  await loadCopy(id);
}

function clearSelection() {
  router.push({ name: "study" });
  selected.value = null;
}

async function refreshActiveCopy() {
  if (!activeCopyId.value) return;
  await loadCopy(activeCopyId.value);
  await loadCopies();
}
</script>

<template>
  <div class="study-shell">
    <header class="study-topbar">
      <PageHeader title="Study Workspace" description="Import PDFs, track answer copies, split questions, and review analytics." />
    </header>

    <main class="study-workspace">
      <aside class="study-sidebar">
        <div class="study-toolbar">
          <input v-model="query" type="search" placeholder="Search copies, candidate, paper" @keyup.enter="loadCopies" />
          <button type="button" @click="loadCopies">Search</button>
        </div>
        <p class="study-summary">{{ summary }} · {{ status }}</p>
        <div class="study-batchbar">
          <button type="button" @click="clearSelection">New Import / Run</button>
        </div>
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
        <template v-if="activeCopyId">
          <StudyWorkflowPanel
            compact
            locked-workflow-id="analyze"
            :review-id="activeCopyId"
            :sync-copy-id="activeCopyId"
            :source-path="selected?.copy.source_path || ''"
            @synced="refreshActiveCopy"
          />
          <StudyQuestionsPanel :detail="selected" />
        </template>
        <StudyWorkflowPanel v-else />
      </section>
    </main>
  </div>
</template>
