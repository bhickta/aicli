<template>
  <div class="config-card">
    <h3 style="margin-bottom: 8px;">Junk Screenshot Sweeper</h3>
    <p class="description">
      Scans a directory using AI. Throws cosmetic icons, logos, or useless graphics into a .trash folder.
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
        {{ pipelineRunning ? 'Running...' : '▶ Start Junk Cleaner' }}
      </button>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.auto_trash" /> Auto Trash
      </label>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.strict" /> Strict Mode (Aggressive)
      </label>
      <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
        <input type="checkbox" v-model="config.sync_refs" /> Sync References
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

