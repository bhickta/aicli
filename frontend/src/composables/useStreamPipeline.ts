import { ref } from 'vue'
import { API_BASE } from '../constants/api.constants'

export interface StreamPipelineOptions {
  /** Build the POST URL from the endpoint string. */
  buildPostUrl: (endpoint: string) => string
  /** Full SSE stream URL. */
  streamUrl: string
  /** Human-readable name for log messages. */
  pipelineName: string
  /** Return true if the config is valid to start. */
  validate: (config: any) => boolean
  /** Validation error message shown when validate returns false. */
  validationMessage?: string
}

export function useStreamPipeline(options: StreamPipelineOptions) {
  const pipelineRunning = ref(false)
  const logs = ref<string[]>([])
  const tasks = ref<Record<string, any>>({})
  let eventSource: EventSource | null = null

  const startPipeline = async (endpoint: string, config: any) => {
    if (!options.validate(config)) {
      alert(options.validationMessage ?? 'Please provide the required fields.')
      return
    }

    logs.value = []
    tasks.value = {}
    pipelineRunning.value = true

    try {
      const res = await fetch(options.buildPostUrl(endpoint), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      })

      if (!res.ok) throw new Error(await res.text())

      if (eventSource) eventSource.close()
      eventSource = new EventSource(options.streamUrl)
      eventSource.onmessage = (e) => {
        const data = JSON.parse(e.data)

        if (data.type === 'status') {
          if (data.status === 'error') {
            logs.value.push(`[SYSTEM ERROR] ${data.message}`)
            pipelineRunning.value = false
          }
          if (data.status === 'completed') {
            logs.value.push(`[SYSTEM] ${options.pipelineName} Pipeline execution completed successfully.`)
            pipelineRunning.value = false
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
      alert('Error starting pipeline: ' + err.message)
      pipelineRunning.value = false
    }
  }

  const cleanup = () => {
    if (eventSource) eventSource.close()
  }

  return { pipelineRunning, logs, tasks, startPipeline, cleanup }
}
