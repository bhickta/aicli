import { API_BASE } from '../constants/api.constants'
import { useStreamPipeline } from './useStreamPipeline'

export function useImagePipeline() {
  const { pipelineRunning, logs, tasks, startPipeline, cleanup } = useStreamPipeline({
    buildPostUrl: (endpoint) => `${API_BASE}/api/image/${endpoint}`,
    streamUrl: `${API_BASE}/api/image/stream`,
    pipelineName: 'Image',
    validate: (config) => !!config.target_path,
    validationMessage: 'Please provide an absolute target path.'
  })

  const startImagePipeline = (endpoint: string, config: any) => startPipeline(endpoint, config)

  return { pipelineRunning, logs, tasks, startImagePipeline, cleanup }
}
