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
      const bestModel = models.find(m => m.includes('31b') || m.includes('70b') || m.includes('qwen') || m.includes('vision')) || models[0];
      const fastModel = models.find(m => m.includes('e4b') || m.includes('8b') || m.includes('9b') || m.includes('mini')) || models[0];
      
      const defaultStepModels: Record<string, string> = props.runConfig.step_models || {};
      if (bestModel !== fastModel && !defaultStepModels['3']) {
        defaultStepModels['3'] = fastModel; // Classification is simple, use fast model
      }

      emit('update:run-config', { ...props.runConfig, llm_model: bestModel, step_models: defaultStepModels });
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

function toggleStepReasoning(stepId: number, value: boolean) {
  const current = props.runConfig.step_reasoning || {};
  emit('update:run-config', {
    ...props.runConfig,
    step_reasoning: { ...current, [stepId.toString()]: value }
  });
}

function setStepModel(stepId: number, modelName: string) {
  const currentModels = props.runConfig.step_models || {};
  const newModels = { ...currentModels };
  if (!modelName) {
    delete newModels[stepId.toString()];
  } else {
    newModels[stepId.toString()] = modelName;
  }
  emit('update:run-config', {
    ...props.runConfig,
    step_models: newModels
  });
}

function handleStart() {
  const config = {
    workers: props.runConfig.workers,
    llm_model: props.runConfig.llm_model,
    allow_reasoning: props.runConfig.allow_reasoning,
    target_steps: props.runConfig.mode === 'all' ? null : props.runConfig.target_steps,
    step_reasoning: props.runConfig.step_reasoning,
    step_models: props.runConfig.step_models
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
            <div v-for="step in PIPELINE_STEPS" :key="step.id" class="step-item" style="display: flex; flex-direction: column; gap: 8px; margin-bottom: 12px; padding: 12px; background: var(--bg-body); border-radius: var(--radius); border: 1px solid var(--border);">
              <div style="display: flex; justify-content: space-between; align-items: center;">
                <label class="checkbox-label" style="display: flex; align-items: center; gap: 8px; font-weight: 600;">
                  <input type="checkbox" :value="step.id" :checked="runConfig.target_steps?.includes(step.id)" @change="toggleStep(step.id)" :disabled="pipelineRunning" />
                  <span>Step {{ step.id }}: {{ step.fullname }}</span>
                </label>
                
                <div style="display: flex; align-items: center; gap: 12px;">
                  <label class="toggle-control" title="Enable/disable reasoning for this step" style="display: flex; align-items: center; gap: 6px; font-size: 11px;">
                    <input type="checkbox" :checked="runConfig.step_reasoning?.[step.id.toString()] ?? true" @change="toggleStepReasoning(step.id, ($event.target as HTMLInputElement).checked)" :disabled="pipelineRunning || !runConfig.allow_reasoning" />
                    Deep Thinking
                  </label>
                  <button class="reset-step-btn" :class="{ disabled: pipelineRunning }" @click.stop="$emit('reset-step', step.id)" style="font-size: 10px; padding: 2px 6px;">↻ Reset</button>
                  <span class="step-badge" v-if="selectedPdf?.progress">{{ selectedPdf.progress[step.id] }}</span>
                </div>
              </div>
              
              <div style="display: flex; align-items: center; gap: 8px; padding-left: 24px;">
                <label style="font-size: 11px; color: var(--text-muted); width: 60px;">Model:</label>
                <select 
                  class="form-select" 
                  style="flex: 1; font-size: 11px; padding: 4px 8px; background: var(--bg-input); border: 1px solid var(--border); color: var(--text-primary); border-radius: 4px;"
                  :value="runConfig.step_models?.[step.id.toString()] || ''"
                  @change="setStepModel(step.id, ($event.target as HTMLSelectElement).value)"
                  :disabled="pipelineRunning"
                >
                  <option value="">Use Default Model</option>
                  <option v-for="m in availableModels" :key="m" :value="m">{{ m }}</option>
                </select>
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
