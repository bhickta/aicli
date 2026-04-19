<template>
  <div class="card">
    <h3 style="margin-bottom: 8px;">GPU Video Compressor</h3>
    <p class="description">
      Compress files blazingly fast by fully saturating an NVENC capable GPU.
    </p>

    <div class="config-grid">
      <div class="form-group span-2">
        <label>Target File/Directory (Absolute Path)</label>
        <input type="text" v-model="config.target_path" placeholder="/home/bhickta/Videos/Raw/" />
      </div>
      
      <div class="form-group">
        <label>Preset</label>
        <select v-model="config.preset">
          <option value="ultralight">ultralight</option>
          <option value="light">light</option>
          <option value="balanced">balanced</option>
          <option value="slideshow">slideshow</option>
        </select>
      </div>

      <div class="form-group">
        <label>Resolution (p)</label>
        <input type="number" v-model.number="config.resolution" min="144" max="1080" />
      </div>
      
      <div class="form-group">
        <label>CRF (Optional Quality Override, 0-51)</label>
        <input type="number" v-model.number="config.crf" min="0" max="51" placeholder="Auto" />
      </div>
      
      <div class="form-group">
        <label>Number of Workers</label>
        <input type="number" v-model.number="config.workers" min="1" max="24" />
      </div>
    </div>

    <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
      <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="$emit('start')" :disabled="pipelineRunning">
        {{ pipelineRunning ? 'Running...' : '▶ Start GPU Compress' }}
      </button>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.overwrite" /> Overwrite Original
      </label>
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
