<template>
  <div class="config-card">
    <h3 style="margin-bottom: 8px;">Markdown Digitize</h3>
    <p class="description">
      Extracts pure-text images into markdown data and replaces file references in your notes natively.
    </p>

    <div class="config-grid">
      <div class="form-group span-full">
        <label>Target Directory (Absolute Path)</label>
        <input type="text" v-model="config.target_path" placeholder="/home/bhickta/Screenshots/" />
      </div>
      
      <div class="form-group">
        <label>LM Vision Workers</label>
        <input type="number" v-model.number="config.workers" min="1" max="16" />
      </div>
    </div>

    <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
      <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="$emit('start')" :disabled="pipelineRunning">
        {{ pipelineRunning ? 'Running...' : '▶ Digitize Text Images' }}
      </button>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.auto_replace" /> Auto Convert & Delete
      </label>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.sync_refs" /> Inject into Markdown
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

