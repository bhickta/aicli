<template>
  <div class="workspace-layout">
    <ImageSidebar v-model:activeTab="activeTab" />

    <main class="main-content">
      <div class="top-bar">
        <h2>{{ tabTitle }}</h2>
      </div>

      <div class="content-body" style="padding: 24px; max-width: 900px;">
        <AIRenamer 
          v-if="activeTab === 'rename'" 
          v-model="renameConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startImagePipeline('rename', renameConfig)" 
        />
        
        <JunkCleaner 
          v-if="activeTab === 'clean'" 
          v-model="cleanConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startImagePipeline('clean', cleanConfig)" 
        />
        
        <MarkdownDigitize 
          v-if="activeTab === 'digitize'" 
          v-model="digitizeConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startImagePipeline('digitize', digitizeConfig)" 
        />

        <LiveTerminal :logs="logs" :tasks="tasks" :pipelineRunning="pipelineRunning" />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { useImagePipeline } from '../composables/useImagePipeline'

import ImageSidebar from './ImageStudio/ImageSidebar.vue'
import AIRenamer from './ImageStudio/AIRenamer.vue'
import JunkCleaner from './ImageStudio/JunkCleaner.vue'
import MarkdownDigitize from './ImageStudio/MarkdownDigitize.vue'
import LiveTerminal from './VideoStudio/LiveTerminal.vue'

const activeTab = ref('rename')
const { pipelineRunning, logs, tasks, startImagePipeline, cleanup } = useImagePipeline()

onBeforeUnmount(cleanup)

const tabTitle = computed(() => {
  if (activeTab.value === 'rename') return 'AI Vision Renamer'
  if (activeTab.value === 'clean') return 'Junk Screenshot Sweeper'
  return 'Markdown Digitizer'
})

const renameConfig = ref({ target_path: '', auto_rename: true, workers: 4, sync_refs: false, trash_junk: true })
const cleanConfig = ref({ target_path: '', auto_trash: true, strict: false, sync_refs: false, workers: 4 })
const digitizeConfig = ref({ target_path: '', auto_replace: true, sync_refs: true, workers: 2 })
</script>
