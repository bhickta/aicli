<template>
  <div class="config-card">
    <h3 style="margin-bottom: 8px;">End-to-End Course Pipeline</h3>
    <p class="description">
      Turns a folder of raw videos into a single merged course pack. Extracts text, uses Whisper for subtitles, generates slideshows, and merges everything into a master video container with embedded CC.
    </p>

    <div class="config-grid config-grid--3col">
      <div class="form-group span-full">
        <label>Target Directory (Absolute Path)</label>
        <div class="select-wrapper">
          <input type="text" v-model="config.target_dir" placeholder="/home/bhickta/Videos/RawCourse" />
          <button @click="browseFolders" class="btn btn-secondary" style="padding: 8px 16px;">📂 Browse</button>
        </div>
      </div>
      
      <div class="form-group">
        <label>Whisper Model (Phase 1)</label>
        <input type="text" v-model="config.whisper_model" />
      </div>
      
      <div class="form-group">
        <label>LLM Tagging Model (Phase 2)</label>
        <div class="select-wrapper">
          <select v-model="config.llm_model" class="form-select">
            <option disabled value="">Select a model...</option>
            <option v-for="m in availableModels" :key="m" :value="m">{{ m }}</option>
            <option v-if="availableModels.length === 0" disabled>No models found</option>
          </select>
          <button @click="refreshModels" class="btn-icon" title="Refresh Models" :disabled="loadingModels">
            <span v-if="!loadingModels">🔄</span>
            <span v-else class="spin">⏳</span>
          </button>
        </div>
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

    <FileExplorer 
      :show="showExplorer" 
      :initialPath="config.target_dir" 
      @close="showExplorer = false" 
      @select="handleSelect" 
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { settingsApi } from '../../api/SettingsApiClient'
import FileExplorer from '../common/FileExplorer.vue'

const props = defineProps<{ modelValue: any, pipelineRunning: boolean }>()
const emit = defineEmits<{ (e: 'update:modelValue', value: any): void, (e: 'start'): void }>()

const config = ref(props.modelValue)
const availableModels = ref<string[]>([])
const loadingModels = ref(false)
const showExplorer = ref(false)

async function refreshModels() {
  loadingModels.value = true
  try {
    const { models } = await settingsApi.fetchModels()
    availableModels.value = models
    // Auto-select first if none selected
    if (models.length > 0 && !config.value.llm_model) {
      config.value.llm_model = models[0]
    }
  } catch (e) {
    console.error('Failed to fetch models:', e)
  } finally {
    loadingModels.value = false
  }
}

function browseFolders() {
  showExplorer.value = true
}

function handleSelect(path: string) {
  config.value.target_dir = path
}

onMounted(refreshModels)
</script>

<style scoped>
.select-wrapper {
  display: flex;
  gap: 8px;
  align-items: center;
}

.form-select {
  flex: 1;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: white;
  padding: 8px 12px;
  border-radius: 6px;
  outline: none;
}

.form-select:focus {
  border-color: #6366f1;
}

.btn-icon {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 16px;
  opacity: 0.7;
  transition: opacity 0.2s;
}

.btn-icon:hover {
  opacity: 1;
}

.spin {
  display: inline-block;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
