<script setup lang="ts">
import { ref, onMounted, nextTick, watch } from 'vue';
import { PIPELINE_STEPS, DEFAULT_STEP_REASONING } from '../../constants/pipeline.constants';

const props = defineProps<{
  pipelineRunning: boolean;
  selectedPdf: any;
  parsedLogs: any[];
  tasks: Record<string, any>;
  autoscroll: boolean;
}>();

const emit = defineEmits<{
  'update:autoscroll': [value: boolean];
  'clear-logs': [];
  'start-pipeline': [config: any];
  'reset-step': [stepId: number];
}>();

const runConfig = ref({
  workers: 4,
  llm_model: 'gemma-4-26b-a4b',
  allow_reasoning: true,
  mode: 'all',
  target_steps: [] as number[],
  stepReasoning: { ...DEFAULT_STEP_REASONING } as Record<number, boolean>
});

const terminalRef = ref<HTMLElement | null>(null);

function toggleStep(stepId: number) {
  if (props.pipelineRunning) return;
  const idx = runConfig.value.target_steps.indexOf(stepId);
  if (idx > -1) runConfig.value.target_steps.splice(idx, 1);
  else {
    runConfig.value.target_steps.push(stepId);
    runConfig.value.target_steps.sort((a, b) => a - b);
  }
}

function toggleStepReasoning(stepId: number) {
  if (props.pipelineRunning || !runConfig.value.allow_reasoning) return;
  runConfig.value.stepReasoning[stepId] = !runConfig.value.stepReasoning[stepId];
}

function handleStart() {
  const config = {
    workers: runConfig.value.workers,
    llm_model: runConfig.value.llm_model,
    allow_reasoning: runConfig.value.allow_reasoning,
    target_steps: runConfig.value.mode === 'all' ? null : runConfig.value.target_steps,
    step_reasoning: runConfig.value.stepReasoning
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
            <input type="number" v-model.number="runConfig.workers" min="1" max="16" :disabled="pipelineRunning" />
          </div>
          <div class="form-group span-full" style="grid-column: span 2;">
            <label>LLM Model ID</label>
            <input type="text" v-model="runConfig.llm_model" placeholder="Model for vision & reasoning" :disabled="pipelineRunning" />
          </div>
          <div class="form-group span-full" style="grid-column: span 2; display: flex; align-items: center; justify-content: center; padding: 12px; background: var(--bg-input); border-radius: var(--radius); margin-top: 8px;">
            <label class="toggle-control" style="display: flex; align-items: center; gap: 10px; cursor: pointer; user-select: none;">
              <input type="checkbox" v-model="runConfig.allow_reasoning" :disabled="pipelineRunning" style="width: 20px; height: 20px;" />
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
              <input type="radio" v-model="runConfig.mode" value="all" :disabled="pipelineRunning" /> End-to-End
            </label>
            <label style="display: flex; align-items: center; gap: 8px;" :style="{ cursor: pipelineRunning ? 'not-allowed' : 'pointer' }">
              <input type="radio" v-model="runConfig.mode" value="custom" :disabled="pipelineRunning" /> Custom Step Selection
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
                  :class="{ active: runConfig.stepReasoning[step.id], disabled: !runConfig.allow_reasoning || pipelineRunning }" 
                  @click.stop="toggleStepReasoning(step.id)"
                  :title="runConfig.stepReasoning[step.id] ? 'Reasoning ENABLED' : 'Reasoning DISABLED'"
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

      <button class="btn btn-primary" @click="handleStart" :disabled="pipelineRunning" style="height: 48px; font-size: 14px; justify-content: center;">
        {{ pipelineRunning ? 'AI Pipeline Working...' : '▶ Start Execution' }}
      </button>
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
