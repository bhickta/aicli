import { ref, computed, onUnmounted } from 'vue'
import { runPipeline, createStream } from '../api.js'

export function useAnalyzePipeline(onCompleted?: () => void) {
  const pipelineRunning = ref(false)
  const logs = ref<string[]>([])
  const tasks = ref<Record<string, any>>({})
  const autoscroll = ref(true)
  let eventSource: EventSource | null = null

  const parsedLogs = computed(() => {
    return logs.value.map(rawMsg => {
      let level = 'info'
      let icon = '⚙️'
      let text = rawMsg
      let page = null

      if (rawMsg.includes('[SUCCESS]')) {
        level = 'success'
        icon = '✅'
        text = rawMsg.replace('[SUCCESS]', '').trim()
      } else if (rawMsg.includes('[ERROR]')) {
        level = 'error'
        icon = '❌'
        text = rawMsg.replace('[ERROR]', '').trim()
      } else if (rawMsg.includes('[PAGE:')) {
        level = 'page'
        icon = '📄'
        const match = rawMsg.match(/\[PAGE:(\d+)\]/)
        if (match) page = match[1]
      } else if (rawMsg.includes('[SYSTEM]')) {
        level = 'system'
        icon = '🚀'
        text = rawMsg.replace('[SYSTEM]', '').trim()
      }

      return { level, icon, text, page, raw: rawMsg }
    })
  })

  function connectStream() {
    if (eventSource) eventSource.close()
    eventSource = createStream()
    
    eventSource.onmessage = (e) => {
      const data = JSON.parse(e.data)
      if (data.type === 'ping') return

      if (data.type === 'status') {
        if (data.status === 'error') {
          logs.value.push(`[SYSTEM ERROR] ${data.message}`)
          pipelineRunning.value = false
        }
        if (data.status === 'completed') {
          logs.value.push(`[SYSTEM] Pipeline execution completed successfully.`)
          pipelineRunning.value = false
          if (onCompleted) onCompleted()
        }
      } else if (data.type === 'task_add') {
        tasks.value[data.task_id] = {
          id: data.task_id,
          description: data.description,
          total: data.total,
          completed: 0
        }
      } else if (data.type === 'task_progress') {
        if (tasks.value[data.task_id]) {
          tasks.value[data.task_id].completed = data.completed
          if (tasks.value[data.task_id].completed >= tasks.value[data.task_id].total) {
            setTimeout(() => delete tasks.value[data.task_id], 1500)
          }
        }
      } else if (data.type === 'log') {
        logs.value.push(data.message)
        if (logs.value.length > 500) logs.value.shift()
      }
    }
  }

  async function startPipeline(config: any) {
    try {
      pipelineRunning.value = true
      logs.value = []
      tasks.value = {}
      connectStream()
      await runPipeline(config)
    } catch (e: any) {
      alert("Could not start pipeline: " + e.message)
      pipelineRunning.value = false
    }
  }

  onUnmounted(() => {
    if (eventSource) eventSource.close()
  })

  return {
    pipelineRunning,
    logs,
    parsedLogs,
    tasks,
    autoscroll,
    startPipeline,
    connectStream
  }
}
