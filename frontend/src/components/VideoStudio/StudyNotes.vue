<template>
  <div class="config-card">
    <h3 style="margin-bottom: 8px;">Markdown Study Notes</h3>
    <p class="description">
      Generate ultra-dense exam-ready study notes from videos with sidecar .srt files or embedded subtitle streams using local LLM inference.
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
        <label>Note Style</label>
        <select v-model="config.style">
          <option value="bullet">bullet (ultra-dense)</option>
          <option value="clean">clean (removes fluff)</option>
        </select>
      </div>
    </div>

    <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
      <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="$emit('start')" :disabled="pipelineRunning">
        {{ pipelineRunning ? 'Running...' : '▶ Generate Notes' }}
      </button>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.overwrite" /> Overwrite Existing
      </label>
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

