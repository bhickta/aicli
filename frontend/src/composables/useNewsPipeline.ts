import { ref } from 'vue'

export function useNewsPipeline() {
  const pipelineRunning = ref(false)
  const logs = ref<string[]>([])
  const tasks = ref<Record<string, any>>({})
  let eventSource: EventSource | null = null

  const startNewsPipeline = async (endpoint: string, config: any) => {
    if (!config.json_path && !config.file_path) {
      alert("Please provide the target file path.")
      return
    }

    logs.value = []
    tasks.value = {}
    pipelineRunning.value = true

    try {
      const res = await fetch(`http://127.0.0.1:8765/api/news/${endpoint}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      })
      
      if (!res.ok) throw new Error(await res.text())
      
      if (eventSource) eventSource.close()
      eventSource = new EventSource('http://127.0.0.1:8765/api/news/stream')
      eventSource.onmessage = (e) => {
        const data = JSON.parse(e.data)
        
        if (data.type === 'status') {
          if (data.status === 'error') {
            logs.value.push(`[SYSTEM ERROR] ${data.message}`)
            pipelineRunning.value = false
          }
          if (data.status === 'completed') {
            logs.value.push(`[SYSTEM] News Pipeline execution completed successfully.`)
            pipelineRunning.value = false
          }
        } else if (data.type === 'task_add') {
          tasks.value[data.task_id] = { id: data.task_id, description: data.description, total: data.total, completed: 0 }
        } else if (data.type === 'task_progress') {
          if (tasks.value[data.task_id]) {
            tasks.value[data.task_id].completed = data.completed
            if (tasks.value[data.task_id].completed >= tasks.value[data.task_id].total) {
              setTimeout(() => { delete tasks.value[data.task_id] }, 1500)
            }
          }
        } else if (data.type === 'log') {
          logs.value.push(data.message)
        }
      }
      eventSource.onerror = () => {
        eventSource?.close()
        pipelineRunning.value = false
      }
    } catch (err: any) {
      alert("Error starting pipeline: " + err.message)
      pipelineRunning.value = false
    }
  }

  const cleanup = () => {
    if (eventSource) eventSource.close()
  }

  return { pipelineRunning, logs, tasks, startNewsPipeline, cleanup }
}
