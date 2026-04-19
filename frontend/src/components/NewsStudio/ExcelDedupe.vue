<template>
  <div class="config-card">
    <h3 style="margin-bottom: 8px;">Excel AI Deduplicator</h3>
    <p class="description">
      Takes an existing Excel file and uses local LLM embeddings to find and merge duplicate rows.
    </p>

    <div class="config-grid">
      <div class="form-group span-full">
        <label>Target Excel File (Absolute Path)</label>
        <input type="text" v-model="config.file_path" placeholder="/home/bhickta/News/master_news.xlsx" />
      </div>
      
      <div class="form-group">
        <label>Output Excel (Optional)</label>
        <input type="text" v-model="config.output" placeholder="Defaults to _deduped.xlsx" />
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

    <div style="margin-top: 24px;">
      <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="$emit('start')" :disabled="pipelineRunning">
        {{ pipelineRunning ? 'Running...' : '▶ Start AI Deduplication' }}
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

