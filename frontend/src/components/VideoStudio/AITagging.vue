<template>
  <div class="card">
    <h3 style="margin-bottom: 8px;">AI Video Tagger</h3>
    <p class="description">
      Transcribe, summarize, and intelligently tag lecture videos using Whisper and LLMs.
    </p>

    <div class="config-grid">
      <div class="form-group span-2">
        <label>Target File/Directory (Absolute Path)</label>
        <input type="text" v-model="config.target_path" placeholder="/home/bhickta/Videos/Raw/" />
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
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
const props = defineProps<{ modelValue: any, pipelineRunning: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: any): void, (e: 'start'): void }>()
const config = ref(props.modelValue)
</script>

<style scoped>
.description { color: var(--text-secondary); font-size: 13px; line-height: 1.5; margin-bottom: 24px; }
.card { background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius-lg); padding: 24px; }
.config-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 16px; }
.span-2 { grid-column: span 3; }
</style>
