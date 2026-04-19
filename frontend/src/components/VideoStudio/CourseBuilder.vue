<template>
  <div class="card">
    <h3 style="margin-bottom: 8px;">End-to-End Course Pipeline</h3>
    <p class="description">
      Turns a folder of raw videos into a single merged course pack. Extracts text, uses Whisper for subtitles, generates slideshows, and merges everything into a master video container with embedded CC.
    </p>

    <div class="config-grid">
      <div class="form-group span-2">
        <label>Target Directory (Absolute Path)</label>
        <input type="text" v-model="config.target_dir" placeholder="/home/bhickta/Videos/RawCourse" />
      </div>
      
      <div class="form-group">
        <label>Whisper Model (Phase 1)</label>
        <input type="text" v-model="config.whisper_model" />
      </div>
      
      <div class="form-group">
        <label>LLM Tagging Model (Phase 2)</label>
        <input type="text" v-model="config.llm_model" />
      </div>

      <div class="form-group">
        <label>Whisper Workers</label>
        <input type="number" v-model.number="config.w1" min="1" max="4" />
      </div>
      <div class="form-group">
        <label>Tag Workers</label>
        <input type="number" v-model.number="config.w2" min="1" max="24" />
      </div>
      <div class="form-group">
        <label>NVENC Workers</label>
        <input type="number" v-model.number="config.w3" min="1" max="24" />
      </div>
    </div>

    <div style="margin-top: 24px;">
      <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="$emit('start')" :disabled="pipelineRunning">
        {{ pipelineRunning ? 'Pipeline Running...' : '▶ Start Compilation' }}
      </button>
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
