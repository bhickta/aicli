<script setup lang="ts">
import { computed, onMounted, watch } from "vue";
import { storeToRefs } from "pinia";
import { useRoute, useRouter } from "vue-router";
import PageHeader from "../components/layout/PageHeader.vue";
import StudyCopyWorkspace from "../components/study/StudyCopyWorkspace.vue";
import StudyRunStatusPanel from "../components/study/StudyRunStatusPanel.vue";
import StudySidebar from "../components/study/StudySidebar.vue";
import { useStudyStore } from "../stores/study";

const route = useRoute();
const router = useRouter();
const study = useStudyStore();
const {
  query,
  copies,
  selected,
  selectedIds,
  batchParallelism,
  forceRerun,
  running,
  activeBatch,
  batchItems,
  runStatus,
  summary,
  visibleStatus,
} = storeToRefs(study);

const activeCopyId = computed(() => selected.value?.copy.id || "");

onMounted(async () => {
  await study.loadCopies();
  const id = String(route.params.copyId || "");
  if (id) await study.openCopy(id);
});

watch(
  () => route.params.copyId,
  async (id) => {
    if (typeof id === "string" && id && id !== activeCopyId.value) await study.openCopy(id);
  },
);

async function openCopy(id: string) {
  await router.push({ name: "study", params: { copyId: id } });
  await study.openCopy(id);
}

function clearSelection() {
  router.push({ name: "study" });
  study.clearSelectedCopy();
}

async function refreshActiveCopy() {
  if (!activeCopyId.value) return;
  await study.openCopy(activeCopyId.value);
  await study.loadCopies();
}
</script>

<template>
  <div class="study-shell">
    <header class="study-topbar">
      <PageHeader title="Study Workspace" description="Import PDFs, analyze topper copies, and review question-wise output." />
    </header>

    <main class="study-workspace">
      <StudySidebar
        v-model:query="query"
        v-model:parallelism="batchParallelism"
        v-model:force-rerun="forceRerun"
        :summary="summary"
        :status="visibleStatus"
        :copies="copies"
        :active-id="activeCopyId"
        :selected-ids="selectedIds"
        :running="running"
        @search="study.loadCopies"
        @clear="clearSelection"
        @run-selected="study.startBatch('all')"
        @open="openCopy"
        @toggle="study.toggleCopy"
      />

      <section class="study-main">
        <StudyRunStatusPanel
          :batch="activeBatch"
          :items="batchItems"
          :status="runStatus"
          :running="running"
        />
        <StudyCopyWorkspace
          :active-copy-id="activeCopyId"
          :detail="selected"
          :force-rerun="forceRerun"
          :running="running"
          @run-copy="study.runCopy"
          @synced="refreshActiveCopy"
        />
      </section>
    </main>
  </div>
</template>
