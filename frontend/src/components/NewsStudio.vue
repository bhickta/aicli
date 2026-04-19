<template>
  <div class="workspace-layout">
    <NewsSidebar v-model:activeTab="activeTab" />

    <main class="main-content">
      <div class="top-bar">
        <h2>{{ tabTitle }}</h2>
      </div>

      <div class="content-body" style="padding: 24px; max-width: 900px;">
        <ProcessFeed 
          v-if="activeTab === 'process'" 
          v-model="processConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startNewsPipeline('process', processConfig)" 
        />
        
        <ExcelDedupe 
          v-if="activeTab === 'dedupe'" 
          v-model="dedupeConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startNewsPipeline('dedupe', dedupeConfig)" 
        />

        <LiveTerminal :logs="logs" :tasks="tasks" :pipelineRunning="pipelineRunning" />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { useNewsPipeline } from '../composables/useNewsPipeline'

import NewsSidebar from './NewsStudio/NewsSidebar.vue'
import ProcessFeed from './NewsStudio/ProcessFeed.vue'
import ExcelDedupe from './NewsStudio/ExcelDedupe.vue'
import LiveTerminal from './VideoStudio/LiveTerminal.vue'

const activeTab = ref('process')
const { pipelineRunning, logs, tasks, startNewsPipeline, cleanup } = useNewsPipeline()

onBeforeUnmount(cleanup)

const tabTitle = computed(() => {
  if (activeTab.value === 'process') return 'God-Mode RAG Pipeline'
  return 'Excel AI Deduplicator'
})

const processConfig = ref({
  json_path: '',
  output: '',
  threshold: 0.82,
  workers: 8,
  no_cache: false
})

const dedupeConfig = ref({
  file_path: '',
  output: '',
  threshold: 0.82,
  workers: 10
})
</script>
