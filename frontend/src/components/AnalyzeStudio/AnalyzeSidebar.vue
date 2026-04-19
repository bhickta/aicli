<script setup lang="ts">
defineProps<{
  pdfs: any[];
  selectedPdf: any | null;
  status: any | null;
  pipelineRunning: boolean;
}>();

defineEmits<{
  'select-pdf': [pdf: any];
  'delete-pdf': [pdf: any];
  'upload-pdfs': [event: Event];
  'retry-errors': [];
  'refresh': [];
}>();
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-header">
      <h1>UPSC Analyzer</h1>
      <div class="subtitle">Topper Answer Sheet Dashboard</div>
    </div>

    <div class="status-bar" v-if="status">
      <span class="status-chip pdfs">{{ status.total_pdfs }} PDFs</span>
      <span class="status-chip pages">{{ status.total_pages }} pages</span>
      <span class="status-chip classified">{{ status.classified_pages }} classified</span>
      <span class="status-chip errors" v-if="status.errors && Object.keys(status.errors).length">
        {{ Object.values(status.errors).reduce((a: number, b: number) => a + b, 0) }} errors
      </span>
    </div>

    <div class="pdf-list">
      <div
        v-for="pdf in pdfs"
        :key="pdf.id"
        :class="['pdf-item', { active: selectedPdf && selectedPdf.id === pdf.id }]"
        @click="$emit('select-pdf', pdf)"
      >
        <span class="icon">📄</span>
        <span class="filename" :title="pdf.filename">{{ pdf.filename }}</span>
        <div class="pdf-progress-dots" v-if="pdf.progress">
          <div 
            v-for="s in ['1','2','3','4','5']" 
            :key="s" 
            :class="['dot', pdf.progress[s]]"
            :title="'Step ' + s + ': ' + pdf.progress[s]"
          ></div>
        </div>
        <span class="page-count" v-if="pdf.page_count">({{ pdf.page_count }}p)</span>
        <button class="delete-btn-mini" @click.stop="$emit('delete-pdf', pdf)" title="Delete PDF" :disabled="pipelineRunning">🗑️</button>
      </div>
      <div v-if="!pdfs.length" class="pdf-item" style="opacity: 0.4; cursor: default;">
        No PDFs found
      </div>
    </div>

    <div class="sidebar-actions">
      <label class="btn btn-ghost btn-sm upload-btn" :class="{ disabled: pipelineRunning }">
        📤 Upload PDFs
        <input type="file" multiple accept=".pdf" @change="$emit('upload-pdfs', $event)" hidden :disabled="pipelineRunning" />
      </label>
      <button class="btn btn-ghost btn-sm" @click="$emit('retry-errors')" :disabled="pipelineRunning">🔄 Retry Error Pages</button>
      <button class="btn btn-ghost btn-sm" @click="$emit('refresh')">↻ Refresh</button>
    </div>
  </aside>
</template>
