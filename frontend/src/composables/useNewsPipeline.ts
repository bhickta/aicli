import { API_BASE } from '../constants/api.constants'
import { useStreamPipeline } from './useStreamPipeline'

export function useNewsPipeline() {
  const { pipelineRunning, logs, tasks, startPipeline, cleanup } = useStreamPipeline({
    buildPostUrl: (endpoint) => `${API_BASE}/api/news/${endpoint}`,
    streamUrl: `${API_BASE}/api/news/stream`,
    pipelineName: 'News',
    validate: (config) => !!(config.json_path || config.file_path),
    validationMessage: 'Please provide the target file path.'
  })

  const startNewsPipeline = (endpoint: string, config: any) => startPipeline(endpoint, config)

  return { pipelineRunning, logs, tasks, startNewsPipeline, cleanup }
}
