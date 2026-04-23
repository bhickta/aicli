<script setup lang="ts">
import { ref, onMounted, nextTick, watch } from 'vue';
import { PIPELINE_STEPS, DEFAULT_STEP_REASONING } from '../../constants/pipeline.constants';

const props = defineProps<{
  pipelineRunning: boolean;
  selectedPdf: any;
  parsedLogs: any[];
  tasks: Record<string, any>;
  autoscroll: boolean;
  runConfig: {
    workers: number;
    llm_model: string;
    allow_reasoning: boolean;
    mode: string;
    target_steps: number[];
    step_reasoning: Record<number, boolean>;
  };
}>();

const emit = defineEmits<{
  'update:autoscroll': [value: boolean];
  'update:run-config': [value: any];
  'clear-logs': [];
  'start-pipeline': [config: any];
  'stop-pipeline': [];
  'reset-step': [stepId: number];
}>();

// local state removed, using props.runConfig
import { settingsApi } from '../../api/SettingsApiClient';

const availableModels = ref<string[]>([]);
const loadingModels = ref(false);

async function refreshModels() {
  loadingModels.value = true;
  try {
    const { models } = await settingsApi.fetchModels();
    availableModels.value = models;
    if (models.length > 0 && !props.runConfig.llm_model) {
      emit('update:run-config', { ...props.runConfig, llm_model: models[0] });
    }
  } catch (e) {
    console.error('Failed to fetch models:', e);
  } finally {
    loadingModels.value = false;
  }
}

onMounted(() => {
  refreshModels();
});

const terminalRef = ref<HTMLElement | null>(null);

function toggleStep(stepId: number) {
  if (props.pipelineRunning) return;
  const config = { ...props.runConfig };
  const idx = config.target_steps.indexOf(stepId);
  if (idx > -1) config.target_steps.splice(idx, 1);
  else {
    config.target_steps.push(stepId);
    config.target_steps.sort((a, b) => a - b);
  }
  emit('update:run-config', config);
}

function toggleStepReasoning(stepId: number) {
  if (props.pipelineRunning || !props.runConfig.allow_reasoning) return;
  const config = { ...props.runConfig };
  config.step_reasoning = { ...config.step_reasoning };
  config.step_reasoning[stepId] = !config.step_reasoning[stepId];
  emit('update:run-config', config);
}

function handleStart() {
  const config = {
    workers: props.runConfig.workers,
    llm_model: props.runConfig.llm_model,
    allow_reasoning: props.runConfig.allow_reasoning,
    target_steps: props.runConfig.mode === 'all' ? null : props.runConfig.target_steps,
    step_reasoning: props.runConfig.step_reasoning
  };
  emit('start-pipeline', config);
}

watch(() => props.parsedLogs.length, () => {
  if (props.autoscroll && terminalRef.value) {
    nextTick(() => {
      terminalRef.value!.scrollTop = terminalRef.value!.scrollHeight;
    });
  }
});
</script>

<template>
  <div class="runner-tab">
    <div class="config-panel">
      <div class="panel-section">
        <h4>Core Parameters</h4>
        <div class="settings-grid">
          <div class="form-group">
            <label>Workers</label>
            <input type="number" :value="runConfig.workers" @input="emit('update:run-config', { ...runConfig, workers: Number(($event.target as HTMLInputElement).value) })" min="1" max="16" :disabled="pipelineRunning" />
          </div>
          <div class="form-group span-full" style="grid-column: span 2;">
            <label>LLM Model (Default)</label>
            <div class="select-wrapper" style="display: flex; gap: 8px; align-items: center;">
              <select 
                class="form-select" 
                style="flex: 1; background: var(--bg-input); border: 1px solid var(--border); color: var(--text-primary); padding: 8px 12px; border-radius: 6px; outline: none;"
                :value="runConfig.llm_model" 
                @change="emit('update:run-config', { ...runConfig, llm_model: ($event.target as HTMLSelectElement).value })"
                :disabled="pipelineRunning"
              >
                <option disabled value="">Select a model...</option>
                <option v-for="m in availableModels" :key="m" :value="m">{{ m }}</option>
                <option v-if="availableModels.length === 0" disabled>No models found</option>
              </select>
              <button @click="refreshModels" class="btn btn-secondary" title="Refresh Models" :disabled="loadingModels" style="padding: 8px;">
                <span v-if="!loadingModels">🔄</span>
                <span v-else class="spin" style="display: inline-block; animation: spin 1s linear infinite;">⏳</span>
              </button>
            </div>
          </div>
          <div class="form-group span-full" style="grid-column: span 2; display: flex; align-items: center; justify-content: center; padding: 12px; background: var(--bg-input); border-radius: var(--radius); margin-top: 8px;">
            <label class="toggle-control" style="display: flex; align-items: center; gap: 10px; cursor: pointer; user-select: none;">
              <input type="checkbox" :checked="runConfig.allow_reasoning" @change="emit('update:run-config', { ...runConfig, allow_reasoning: ($event.target as HTMLInputElement).checked })" :disabled="pipelineRunning" style="width: 20px; height: 20px;" />
              <span style="font-weight: 600; font-size: 13px; color: var(--text-primary);">Model Reasoning (Master Toggle)</span>
            </label>
            <div class="info-tip" style="margin-left: 8px;" title="Master switch for Deep Thinking.">ⓘ</div>
          </div>
        </div>
      </div>

      <div class="panel-section">
        <h4>
          Pipeline Workflow
          <button class="btn btn-ghost btn-sm btn-danger" @click="$emit('reset-step', 1)" :disabled="pipelineRunning" style="font-size: 10px; padding: 2px 8px;">
            Reset All Data
          </button>
        </h4>
        <div class="form-group">
          <div class="radio-group" style="display: flex; gap: 16px; margin-bottom: 8px;">
            <label style="display: flex; align-items: center; gap: 8px;" :style="{ cursor: pipelineRunning ? 'not-allowed' : 'pointer' }">
              <input type="radio" :checked="runConfig.mode === 'all'" @change="emit('update:run-config', { ...runConfig, mode: 'all' })" name="run_mode" :disabled="pipelineRunning" /> End-to-End
            </label>
            <label style="display: flex; align-items: center; gap: 8px;" :style="{ cursor: pipelineRunning ? 'not-allowed' : 'pointer' }">
              <input type="radio" :checked="runConfig.mode === 'custom'" @change="emit('update:run-config', { ...runConfig, mode: 'custom' })" name="run_mode" :disabled="pipelineRunning" /> Custom Step Selection
            </label>
          </div>
          
          <div v-if="runConfig.mode === 'custom'" class="steps-list">
            <div 
              v-for="step in PIPELINE_STEPS" 
              :key="step.id" 
              class="step-row"
              :class="[selectedPdf.progress ? selectedPdf.progress[step.id] : '', { disabled: pipelineRunning }]"
              @click="toggleStep(step.id)"
            >
              <input 
                type="checkbox" 
                :value="step.id" 
                :checked="runConfig.target_steps.includes(step.id)" 
                @click.stop="toggleStep(step.id)"
                :disabled="pipelineRunning" 
              />
              <span class="step-name">{{ step.id }}: {{ step.fullname }}</span>
              
              <div style="display: flex; align-items: center; gap: 8px;">
                <div 
                  v-if="step.id > 1"
                  class="reasoning-toggle" 
                  :class="{ active: runConfig.step_reasoning[step.id], disabled: !runConfig.allow_reasoning || pipelineRunning }" 
                  @click.stop="toggleStepReasoning(step.id)"
                  :title="runConfig.step_reasoning[step.id] ? 'Reasoning ENABLED' : 'Reasoning DISABLED'"
                >
                  🧠
                </div>
                <button class="reset-step-btn" :class="{ disabled: pipelineRunning }" @click.stop="$emit('reset-step', step.id)">↻ Reset</button>
                <span class="step-badge" v-if="selectedPdf.progress">{{ selectedPdf.progress[step.id] }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="action-buttons-v2">
        <button v-if="pipelineRunning" class="btn btn-danger stop-btn-lg" @click="$emit('stop-pipeline')">
          ⏹ STOP PIPELINE EXECUTION
        </button>
        <button v-else class="btn btn-primary start-btn-lg" @click="handleStart">
          ▶ Start Execution
        </button>
        
        <div v-if="pipelineRunning" class="working-pill">
          <span class="spinner-sm"></span>
          AI Pipeline is working...
        </div>
      </div>
    </div>

    <div class="console-panel">
      <div class="console-header">
        <h3>Live Execution Logs</h3>
        <div class="console-controls">
          <label class="autoscroll-toggle">
            <input type="checkbox" :checked="autoscroll" @change="$emit('update:autoscroll', ($event.target as HTMLInputElement).checked)" /> Auto-scroll
          </label>
          <button class="clear-btn" @click="$emit('clear-logs')">🗑️ Clear</button>
        </div>
      </div>
      
      <div class="tasks-overlay" v-if="Object.keys(tasks).length > 0">
        <div v-for="task in tasks" :key="task.id" class="task-bar-container">
          <div class="task-info">
            <span>{{ task.description }}</span>
            <span>{{ task.completed.toFixed(0) }} / {{ task.total }}</span>
          </div>
          <div class="progress-bar">
            <div class="fill" :style="{ width: Math.min(100, Math.max(0, (task.completed / task.total) * 100)) + '%' }"></div>
          </div>
        </div>
      </div>
      
      <div class="terminal" ref="terminalRef">
        <div v-for="(log, i) in parsedLogs" :key="i" :class="['log-line', log.level]">
          <span class="log-icon">{{ log.icon }}</span>
          <span class="log-text">{{ log.text }}</span>
          <span v-if="log.page" class="log-page-tag">Page {{ log.page }}</span>
        </div>
        <div v-if="!parsedLogs.length" class="empty-terminal">
          _ Waiting for pipeline execution...
        </div>
      </div>
    </div>
  </div>
</template>
