<template>
  <div class="studio-container">
    <div class="studio-header">
      <h2>Global System Settings</h2>
      <p>Configure backend API endpoints, LLM provider settings, and global concurrency behaviors here.</p>
    </div>

    <div class="studio-content">
      <div class="tabs-header">
        <button 
          v-for="tab in tabs" 
          :key="tab.id"
          class="tab-btn" 
          :class="{ active: activeTab === tab.id }"
          @click="activeTab = tab.id"
        >
          {{ tab.label }}
        </button>
      </div>

      <div class="config-card">
        <!-- 0. LLM Provider Settings -->
        <template v-if="activeTab === 'provider'">
          <h3 style="margin-bottom: 8px;">LLM Provider Selection</h3>
          <p class="description">
            Choose the primary intelligence backend for vision and text reasoning.
          </p>
          <div class="form-group span-full">
            <label>Platform</label>
            <select v-model="settings.provider_type" class="provider-select">
              <option v-for="prov in providers" :key="prov" :value="prov">
                {{ prov.toUpperCase() }}
              </option>
            </select>
          </div>

          <hr style="margin: 24px 0; border: none; border-top: 1px solid var(--border-color);" />

          <h3 style="margin-bottom: 8px;">Advanced Configuration & Tuning</h3>
          <p class="description">
            Fine-tune pipeline thresholds, LLM parameters, and system timeouts.
          </p>

          <div class="config-grid">
            <!-- Ollama Settings -->
            <template v-if="settings.provider_type === 'ollama'">
              <div class="form-group span-full">
                <label>Ollama Base URL</label>
                <input type="text" v-model="settings.ollama_base_url" placeholder="http://localhost:11434" />
              </div>
            </template>

            <!-- vLLM Settings -->
            <template v-if="settings.provider_type === 'vllm'">
              <div class="form-group span-full">
                <label>vLLM Base URL</label>
                <input type="text" v-model="settings.vllm_base_url" placeholder="http://localhost:8000" />
              </div>
              <div class="form-group">
                <label>API Key</label>
                <input type="text" v-model="settings.vllm_api_key" placeholder="EMPTY" />
              </div>
            </template>

            <!-- LM Studio Settings -->
            <template v-if="settings.provider_type === 'lmstudio'">
              <div class="form-group span-full">
                <label>LM Studio Base URL</label>
                <input type="text" v-model="settings.lm_studio_base_url" placeholder="http://localhost:1234/v1" />
              </div>
              <div class="form-group">
                <label>API Key</label>
                <input type="text" v-model="settings.lm_studio_api_key" placeholder="lm_studio" />
              </div>
            </template>

            <!-- OpenAI Settings -->
            <template v-if="settings.provider_type === 'openai'">
              <div class="form-group span-full">
                <label>OpenAI API Key</label>
                <input type="password" v-model="settings.openai_api_key" placeholder="sk-proj-..." />
              </div>
            </template>

            <!-- Anthropic Settings -->
            <template v-if="settings.provider_type === 'anthropic'">
              <div class="form-group span-full">
                <label>Anthropic API Key</label>
                <input type="password" v-model="settings.anthropic_api_key" placeholder="sk-ant-..." />
              </div>
            </template>

            <!-- Gemini Settings -->
            <template v-if="settings.provider_type === 'gemini'">
              <div class="form-group span-full">
                <label>Gemini API Key</label>
                <input type="password" v-model="settings.gemini_api_key" placeholder="AIzaSy..." />
              </div>
            </template>

            <div class="form-group span-full" style="margin-top: 16px;">
              <label>Preferred Base Model (Global Default)</label>
              <div class="select-wrapper" style="display: flex; gap: 8px; align-items: center;">
                <select 
                  class="form-select" 
                  style="flex: 1; background: var(--bg-input); border: 1px solid var(--border); color: var(--text-primary); padding: 8px 12px; border-radius: 6px; outline: none;"
                  v-model="settings.model_name"
                >
                  <option disabled value="">Select a model...</option>
                  <option v-for="m in availableModels" :key="m" :value="m">{{ m }}</option>
                  <option v-if="availableModels.length === 0" disabled>No models found. Ensure LLM is running.</option>
                </select>
                <button @click="refreshModels" class="btn btn-secondary" title="Refresh Models" :disabled="loadingModels" style="padding: 8px;">
                  <span v-if="!loadingModels">🔄</span>
                  <span v-else class="spin" style="display: inline-block; animation: spin 1s linear infinite;">⏳</span>
                </button>
              </div>
            </div>
          </div>
        </template>

        <!-- 1. UPSC Analyze Settings -->
        <template v-if="activeTab === 'analyze'">
          <h3 style="margin-bottom: 8px; color: var(--accent);">UPSC Analyze Settings</h3>
          <p class="description" style="margin-bottom: 16px;">
            Configuration for the answer extraction and grading pipeline.
          </p>
          <div class="config-grid">
            <div class="form-group">
              <label>Analyze Max Tokens</label>
              <input type="number" v-model.number="settings.analyze_max_tokens" />
            </div>
            <div class="form-group">
              <label>Analyze Temp</label>
              <input type="number" step="0.1" v-model.number="settings.analyze_temperature" />
            </div>
            <div class="form-group">
              <label>Segmenter Max Tokens</label>
              <input type="number" v-model.number="settings.segmenter_max_tokens" />
            </div>
            <div class="form-group">
              <label>Segmenter Retries</label>
              <input type="number" v-model.number="settings.segmenter_max_retries" />
            </div>
            <div class="form-group">
              <label>Agg. Chunk Size</label>
              <input type="number" v-model.number="settings.aggregation_chunk_size" />
            </div>
            <div class="form-group span-full" style="margin-top: 16px; background: var(--bg-surface); padding: 16px; border-radius: 8px; border: 1px solid var(--border-color);">
              <h4 style="font-size: 14px; font-weight: 600; color: var(--text-primary); margin-bottom: 12px;">Step-wise Model Overrides</h4>
              <p style="font-size: 12px; color: var(--text-secondary); margin-bottom: 16px;">Optionally assign a specific model to individual pipeline steps. Leave as 'Use Global Default' to use the Preferred Base Model.</p>
              
              <div v-for="step in PIPELINE_STEPS" :key="step.id" style="display: flex; gap: 12px; margin-bottom: 8px; align-items: center;">
                <span style="font-size: 13px; font-weight: 500; width: 180px; color: var(--text-secondary);">Step {{ step.id }}: {{ step.fullname || step.name }}</span>
                <select class="form-select" style="flex: 1; padding: 6px 10px; font-size: 13px; background: var(--bg-input); border: 1px solid var(--border-color); color: var(--text-primary); border-radius: 4px;" v-model="settings.analyze_step_models[step.id.toString()]">
                  <option value="">Use Global Default</option>
                  <option v-for="m in availableModels" :key="m" :value="m">{{ m }}</option>
                </select>
              </div>
            </div>
          </div>
        </template>

        <!-- 2. Video Notes Settings -->
        <template v-if="activeTab === 'video'">
          <h3 style="margin-bottom: 8px; color: var(--accent);">Video Notes Settings</h3>
          <p class="description" style="margin-bottom: 16px;">
            Configuration for audio transcription and YouTube note generation.
          </p>
          <div class="config-grid">
            <div class="form-group">
              <label>Notes Temp</label>
              <input type="number" step="0.1" v-model.number="settings.notes_temperature" />
            </div>
            <div class="form-group">
              <label>Notes Max Tokens</label>
              <input type="number" v-model.number="settings.notes_max_tokens" />
            </div>
            <div class="form-group">
              <label>Notes Chunk (Chars)</label>
              <input type="number" v-model.number="settings.notes_chunk_size" />
            </div>
            <div class="form-group">
              <label>Whisper Batch Size</label>
              <input type="number" v-model.number="settings.whisper_batch_size" />
            </div>
          </div>
        </template>

        <!-- 3. News & Curation Settings -->
        <template v-if="activeTab === 'news'">
          <h3 style="margin-bottom: 8px; color: var(--accent);">News & Curation Settings</h3>
          <p class="description" style="margin-bottom: 16px;">
            Configuration for the RSS news summarization pipeline.
          </p>
          <div class="config-grid">
            <div class="form-group">
              <label>News Batch Size</label>
              <input type="number" v-model.number="settings.news_batch_size" />
            </div>
            <div class="form-group">
              <label>News Merge Temp</label>
              <input type="number" step="0.1" v-model.number="settings.news_merge_temperature" />
            </div>
          </div>
        </template>

        <!-- 4. Global System Settings -->
        <template v-if="activeTab === 'system'">
          <h3 style="margin-bottom: 8px; color: var(--accent);">Global System Settings</h3>
          <p class="description" style="margin-bottom: 16px;">
            Base LLM fallback parameters and database timeouts.
          </p>
          <div class="config-grid">
            <div class="form-group">
              <label>Global Default Temp</label>
              <input type="number" step="0.1" v-model.number="settings.llm_default_temperature" />
            </div>
            <div class="form-group">
              <label>Global Default Retries</label>
              <input type="number" v-model.number="settings.llm_default_max_retries" />
            </div>
            <div class="form-group">
              <label>DB Conn Timeout (s)</label>
              <input type="number" v-model.number="settings.db_connect_timeout" />
            </div>
            <div class="form-group">
              <label>DB Busy Timeout (ms)</label>
              <input type="number" v-model.number="settings.db_busy_timeout_ms" />
            </div>
          </div>
        </template>

        <div style="margin-top: 32px; padding-top: 24px; border-top: 1px solid var(--border-color);">
          <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px; width: 100%;" @click="saveSettings">
            Save Global Configuration
          </button>
        </div>
        <!-- Toast Notification -->
        <div v-if="showSuccess" class="toast success">
          ✓ Settings successfully updated
        </div>
        <div v-if="errorMessage" class="toast error">
          ⚠ {{ errorMessage }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { API_BASE } from '../constants/api.constants'
import { settingsApi } from '../api/SettingsApiClient'
import { PIPELINE_STEPS } from '../constants/pipeline.constants'

const showSuccess = ref(false)
const errorMessage = ref('')

const activeTab = ref('provider')
const tabs = [
  { id: 'provider', label: 'LLM Provider' },
  { id: 'analyze', label: 'UPSC Analyze' },
  { id: 'video', label: 'Video Notes' },
  { id: 'news', label: 'News & Curation' },
  { id: 'system', label: 'Global System' }
]

const providers = ref<string[]>([])
const availableModels = ref<string[]>([])
const loadingModels = ref(false)

const settings = ref({
  provider_type: 'ollama',
  model_name: 'qwen3.5-9b',
  ollama_base_url: 'http://localhost:11434',
  ollama_api_key: 'ollama',
  vllm_base_url: 'http://localhost:8000',
  vllm_api_key: 'EMPTY',
  lm_studio_base_url: 'http://localhost:1234/v1',
  lm_studio_api_key: 'lm_studio',
  openai_api_key: '',
  anthropic_api_key: '',
  gemini_api_key: '',
  analyze_step_models: {} as Record<string, string>
})

async function refreshModels() {
  loadingModels.value = true;
  try {
    const { models } = await settingsApi.fetchModels();
    availableModels.value = models;
  } catch (e) {
    console.error('Failed to fetch models:', e);
  } finally {
    loadingModels.value = false;
  }
}

async function loadProviders() {
  try {
    const res = await fetch(`${API_BASE}/api/settings/providers`)
    if (res.ok) {
      const data = await res.json()
      providers.value = data.providers
    }
  } catch (err) {
    console.error('Failed to load providers:', err)
  }
}

async function loadSettings() {
  try {
    const res = await fetch(`${API_BASE}/api/settings`)
    if (res.ok) {
      const data = await res.json()
      if (!data.analyze_step_models) {
        data.analyze_step_models = {}
      }
      settings.value = data
    }
  } catch (err) {
    console.error('Failed to load settings:', err)
  }
}

async function saveSettings() {
  try {
    const res = await fetch(`${API_BASE}/api/settings`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
    })
    if (res.ok) {
      showSuccess.value = true
      setTimeout(() => showSuccess.value = false, 3000)
    } else {
      errorMessage.value = 'Failed to update settings.'
      setTimeout(() => errorMessage.value = '', 3000)
    }
  } catch (err: any) {
    errorMessage.value = 'Error saving settings: ' + err.message
    setTimeout(() => errorMessage.value = '', 3000)
  }
}

onMounted(async () => {
  await loadProviders()
  await loadSettings()
  await refreshModels()
})
</script>

<style scoped>
.studio-container {
  padding: 32px 48px;
  max-width: 1200px;
  margin: 0 auto;
}
.studio-header h2 { margin: 0 0 8px; font-size: 28px; font-weight: 600; }
.studio-header p { margin: 0; color: var(--text-secondary); font-size: 15px; }
.studio-content { margin-top: 32px; display: flex; flex-direction: column; gap: 24px; }
.tabs-header {
  display: flex;
  gap: 8px;
  border-bottom: 1px solid var(--border-color);
  padding-bottom: 0;
  margin-bottom: -8px;
  overflow-x: auto;
}
.tab-btn {
  padding: 12px 24px;
  background: transparent;
  border: none;
  border-bottom: 3px solid transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}
.tab-btn:hover {
  color: var(--text-primary);
  background: rgba(255,255,255,0.05);
}
.tab-btn.active {
  color: var(--accent);
  border-bottom-color: var(--accent);
}
.provider-select {
  width: 100%;
  padding: 8px 12px;
  border-radius: 4px;
  border: 1px solid var(--border-color);
  background: var(--bg-surface);
  color: var(--text-primary);
  font-size: 14px;
  margin-top: 8px;
}
.toast {
  position: fixed;
  bottom: 32px;
  right: 32px;
  padding: 12px 24px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  box-shadow: 0 4px 12px rgba(0,0,0,0.15);
  animation: slideIn 0.3s ease;
  z-index: 1000;
}
.toast.success {
  background: var(--bg-card);
  color: #4ade80;
  border: 1px solid #4ade80;
}
.toast.error {
  background: var(--bg-card);
  color: #f87171;
  border: 1px solid #f87171;
}
@keyframes slideIn {
  from { transform: translateY(100%); opacity: 0; }
  to { transform: translateY(0); opacity: 1; }
}
</style>
