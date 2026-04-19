<script setup lang="ts">
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

const pipelineSteps = [
  { id: 1, name: 'Images' },
  { id: 2, name: 'OCR' },
  { id: 3, name: 'Classify' },
  { id: 4, name: 'Segment' },
  { id: 5, name: 'Analyze' },
  { id: 6, name: 'Aggregate' },
  { id: 7, name: 'Report' }
];
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
      <div v-for="step in pipelineSteps" :key="step.id" :class="['status-step', selectedPdf.progress[step.id]]">
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
