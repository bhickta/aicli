<template>
  <div class="workspace-layout">
    <VideoSidebar v-model:activeTab="activeTab" />

    <main class="main-content">
      <div class="top-bar">
        <h2>{{ tabTitle }}</h2>
      </div>

      <div class="content-body" style="padding: 24px; max-width: 900px;">
        <CourseBuilder 
          v-if="activeTab === 'course'" 
          v-model="courseConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startVideoPipeline('course', courseConfig)" 
        />
        
        <GPUCompress 
          v-if="activeTab === 'compress'" 
          v-model="compressConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startVideoPipeline('compress', compressConfig)" 
        />
        
        <AITagging 
          v-if="activeTab === 'tag'" 
          v-model="tagConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startVideoPipeline('tag', tagConfig)" 
        />
        
        <StudyNotes 
          v-if="activeTab === 'notes'" 
          v-model="notesConfig" 
          :pipelineRunning="pipelineRunning" 
          @start="startVideoPipeline('notes', notesConfig)" 
        />

        <LiveTerminal :logs="logs" :tasks="tasks" :pipelineRunning="pipelineRunning" />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { useVideoPipeline } from '../composables/useVideoPipeline'

import VideoSidebar from './VideoStudio/VideoSidebar.vue'
import CourseBuilder from './VideoStudio/CourseBuilder.vue'
import GPUCompress from './VideoStudio/GPUCompress.vue'
import AITagging from './VideoStudio/AITagging.vue'
import StudyNotes from './VideoStudio/StudyNotes.vue'
import LiveTerminal from './VideoStudio/LiveTerminal.vue'

const activeTab = ref('course')
const { pipelineRunning, logs, tasks, startVideoPipeline, cleanup } = useVideoPipeline()

onBeforeUnmount(cleanup)

const tabTitle = computed(() => {
  if (activeTab.value === 'course') return 'Course Compilation'
  if (activeTab.value === 'compress') return 'NVENC Compression'
  if (activeTab.value === 'notes') return 'Markdown Study Notes'
  return 'Video Tagging'
})

const courseConfig = ref({
  target_dir: '', whisper_model: 'large-v3', cleanup: 'keep',
  w1: 2, w2: 12, w3: 12, llm_model: 'gemma-4-26b-a4b'
})

const compressConfig = ref({
  target_path: '', preset: 'light', resolution: 240,
  overwrite: false, workers: 4, crf: null
})

const tagConfig = ref({
  target_path: '', whisper_model: 'base', write: false,
  transcribe_only: false, full_cc: false, retranscribe: false, workers: 2
})

const notesConfig = ref({ target_path: '', style: 'bullet', overwrite: false })
</script>
