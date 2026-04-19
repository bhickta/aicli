<script setup lang="ts">
import { PIPELINE_STEPS } from '../../constants/pipeline.constants';

const props = defineProps<{
  selectedPdf: any;
  activeTab: string;
  pipelineRunning: boolean;
  answersCount: number;
}>();

defineEmits<{
  'update:activeTab': [tab: string];
  'delete-pdf': [pdf: any];
}>();
</script>

<template>
  <div class="top-bar">
    <div class="top-bar-title">
      <h2>{{ selectedPdf.filename }}</h2>
      <button class="btn btn-ghost btn-sm btn-danger" @click="$emit('delete-pdf', selectedPdf)" :disabled="pipelineRunning">
        🗑️ Delete PDF
      </button>
    </div>
    
    <div class="pdf-status-strip" v-if="selectedPdf.progress">
      <div v-for="step in PIPELINE_STEPS" :key="step.id" :class="['status-step', selectedPdf.progress[step.id]]">
        <div class="step-dot"></div>
        <span>{{ step.name }}</span>
      </div>
    </div>

    <div class="tabs">
      <button :class="['tab', { active: activeTab === 'answers' }]" @click="$emit('update:activeTab', 'answers')">
        Questions ({{ answersCount }})
      </button>
      <button :class="['tab', { active: activeTab === 'aggregation' }]" @click="$emit('update:activeTab', 'aggregation')">
        Aggregation
      </button>
      <button :class="['tab', { active: activeTab === 'runner' }]" @click="$emit('update:activeTab', 'runner')" class="runner-btn">
        ▶ Runner
      </button>
    </div>
  </div>
</template>
