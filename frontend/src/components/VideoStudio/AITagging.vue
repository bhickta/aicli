<template>
  <div class="config-card">
    <h3 style="margin-bottom: 8px;">AI Video Tagger</h3>
    <p class="description">
      Transcribe, summarize, and intelligently tag lecture videos using Whisper and LLMs.
    </p>

    <div class="config-grid config-grid--3col">
      <div class="form-group span-full">
        <label>Target File/Directory (Absolute Path)</label>
        <div class="select-wrapper">
          <input type="text" v-model="config.target_path" placeholder="/home/bhickta/Videos/Raw/" />
          <button @click="showExplorer = true" class="btn btn-secondary" style="padding: 8px 16px;">📂 Browse</button>
        </div>
      </div>
      
      <div class="form-group">
        <label>Whisper Model</label>
        <input type="text" v-model="config.whisper_model" />
      </div>

      <div class="form-group">
         <label>Workers</label>
         <input type="number" v-model.number="config.workers" min="1" max="24" />
      </div>
    </div>

    <div style="margin-top: 24px; display: flex; flex-wrap: wrap; gap: 16px; align-items: center;">
      <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="$emit('start')" :disabled="pipelineRunning">
        {{ pipelineRunning ? 'Running...' : '▶ Start AI Tagging' }}
      </button>
      
      <div style="display: flex; flex-direction: column; gap: 6px;">
        <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
          <input type="checkbox" v-model="config.write" /> Write Mode (Rename Files)
        </label>
        <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
          <input type="checkbox" v-model="config.transcribe_only" /> Transcribe Only
        </label>
      </div>
      <div style="display: flex; flex-direction: column; gap: 6px;">
        <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
          <input type="checkbox" v-model="config.full_cc" /> Generate Subtitles (.srt)
        </label>
        <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
          <input type="checkbox" v-model="config.retranscribe" /> Force Retranscribe
        </label>
      </div>
    </div>

    <FileExplorer 
      :show="showExplorer" 
      :initialPath="config.target_path" 
      @close="showExplorer = false" 
      @select="config.target_path = $event" 
    />
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import FileExplorer from '../common/FileExplorer.vue'

const props = defineProps<{ modelValue: any, pipelineRunning: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: any): void, (e: 'start'): void }>()
const config = ref(props.modelValue)
const showExplorer = ref(false)
</script>

<style scoped>
.select-wrapper {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>

