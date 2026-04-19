import { API_BASE } from '../constants/api.constants'
import { useStreamPipeline } from './useStreamPipeline'

export function useVideoPipeline() {
  const { pipelineRunning, logs, tasks, startPipeline, cleanup } = useStreamPipeline({
    buildPostUrl: (endpoint) => `${API_BASE}/api/video/${endpoint}/run`,
    streamUrl: `${API_BASE}/api/video/course/stream`,
    pipelineName: 'Video',
    validate: (config) => !!(config.target_path || config.target_dir),
    validationMessage: 'Please provide an absolute target path.'
  })

  const startVideoPipeline = (endpoint: string, config: any) => startPipeline(endpoint, config)

  return { pipelineRunning, logs, tasks, startVideoPipeline, cleanup }
}
