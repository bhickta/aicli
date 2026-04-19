<template>
  <div class="studio-container">
    <div class="studio-header">
      <h2>Global System Settings</h2>
      <p>Configure backend API endpoints, LLM provider settings, and global concurrency behaviors here.</p>
    </div>

    <div class="studio-content">
      <div class="config-card">
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
            <label>Preferred Base Model</label>
            <input type="text" v-model="settings.model_name" placeholder="qwen3.5-9b" />
          </div>
        </div>

        <div style="margin-top: 24px;">
          <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="saveSettings">
            Save Configuration
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { API_BASE } from '../constants/api.constants'

const providers = ref<string[]>([])
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
  gemini_api_key: ''
})

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
      settings.value = await res.json()
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
      alert('Settings successfully updated across the global system.')
    } else {
      alert('Failed to update settings.')
    }
  } catch (err: any) {
    alert('Error saving settings: ' + err.message)
  }
}

onMounted(async () => {
  await loadProviders()
  await loadSettings()
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
</style>
