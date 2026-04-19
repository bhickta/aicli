<template>
  <div class="card">
    <h3 style="margin-bottom: 8px;">Parse & Deduplicate JSON Feed</h3>
    <p class="description">
      Parses a raw JSON news feed, appends it into an existing master Excel database, and natively deduplicates the entire dataset using RAG + Local LLM.
    </p>

    <div class="config-grid">
      <div class="form-group span-2">
        <label>Input JSON File (Absolute Path)</label>
        <input type="text" v-model="config.json_path" placeholder="/home/bhickta/News/december.json" />
      </div>
      
      <div class="form-group span-2">
        <label>Output Master Excel (Absolute Path)</label>
        <input type="text" v-model="config.output" placeholder="/home/bhickta/News/master_news.xlsx" />
      </div>

      <div class="form-group">
        <label>Cosine Threshold</label>
        <input type="number" step="0.05" min="0.5" max="1.0" v-model.number="config.threshold" />
      </div>
      
      <div class="form-group">
        <label>LLM Merge Workers</label>
        <input type="number" v-model.number="config.workers" min="1" max="24" />
      </div>
    </div>

    <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
      <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="$emit('start')" :disabled="pipelineRunning">
        {{ pipelineRunning ? 'Pipeline Running...' : '▶ Start End-to-End Build' }}
      </button>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.no_cache" /> Disable Cache
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
.config-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.span-2 { grid-column: span 2; }
</style>
