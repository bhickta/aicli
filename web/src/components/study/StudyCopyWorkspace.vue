<script setup lang="ts">
import type { StudyCopyDetail } from "../../types";
import StudyQuestionsPanel from "./StudyQuestionsPanel.vue";
import StudyWorkflowPanel from "./StudyWorkflowPanel.vue";

defineProps<{
  activeCopyId: string;
  detail: StudyCopyDetail | null;
  forceRerun: boolean;
  running: boolean;
}>();

const emit = defineEmits<{
  runCopy: [id: string];
  synced: [];
}>();
</script>

<template>
  <section class="study-copy-workspace">
    <template v-if="activeCopyId">
      <div class="study-copy-actions">
        <button type="button" :disabled="running" @click="emit('runCopy', activeCopyId)">
          {{ running ? "Analysis running..." : forceRerun ? "Bypass cache and analyze" : "Analyze PDF" }}
        </button>
      </div>
      <StudyWorkflowPanel
        compact
        locked-workflow-id="analyze"
        :review-id="activeCopyId"
        :sync-copy-id="activeCopyId"
        :source-path="detail?.copy.source_path || ''"
        @synced="emit('synced')"
      />
      <StudyQuestionsPanel :detail="detail" />
    </template>
    <StudyWorkflowPanel v-else />
  </section>
</template>
