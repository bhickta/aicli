<template>
  <div class="console-panel" style="margin-top: 24px;" v-if="logs.length > 0 || pipelineRunning">
    <div class="console-header">
      <h3>Live Execution Logs</h3>
    </div>
    
    <div class="tasks-overlay" v-if="Object.keys(tasks).length > 0">
      <div v-for="task in tasks" :key="task.id" class="task-bar-container">
        <div class="task-info">
          <span>{{ task.description }}</span>
        </div>
      </div>
    </div>
    
    <div class="terminal" ref="terminalRef" style="height: 400px;">
      <div v-for="(log, i) in logs" :key="i" class="log-line">
        <span v-if="log.includes('[SUCCESS]')" style="color: var(--success);">{{ log }}</span>
        <span v-else-if="log.includes('[ERROR]')" style="color: var(--danger);">{{ log }}</span>
        <span v-else>{{ log }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'

const props = defineProps<{ logs: string[], tasks: Record<string, any>, pipelineRunning: boolean }>()
const terminalRef = ref<HTMLElement | null>(null)

watch(() => props.logs.length, () => {
  nextTick(() => {
    if (terminalRef.value) {
      terminalRef.value.scrollTop = terminalRef.value.scrollHeight
    }
  })
})
</script>
