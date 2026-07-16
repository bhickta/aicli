export interface ProviderConfig {
  id: string;
  type: string;
  name: string;
  base_url: string;
  api_key: string;
  api_key_env?: string;
  model: string;
  model_filter?: string;
  reasoning_effort?: string;
  text_verbosity?: string;
  prompt_cache_key?: string;
  prompt_cache_retention?: string;
  headers?: Record<string, string>;
}

export interface Settings {
  default_provider: string;
  default_model: string;
  providers: ProviderConfig[];
  execution_profiles: ExecutionProfile[];
  tools: Record<string, string>;
}

export interface ExecutionTarget {
  provider_id: string;
  model: string;
  priority: number;
  enabled: boolean;
  input_cost_per_million: number;
  output_cost_per_million: number;
}

export interface ExecutionProfile {
  id: string;
  name: string;
  capability: string;
  enabled: boolean;
  max_concurrency: number;
  timeout_seconds: number;
  cooldown_seconds: number;
  targets: ExecutionTarget[];
}

export interface SystemResources {
  collected_at: string;
  cpu: {
    logical_cores: number;
    usage_percent: number;
    load_1: number;
    load_5: number;
    load_15: number;
  };
  ram: {
    total_bytes: number;
    available_bytes: number;
    used_bytes: number;
    usage_percent: number;
  };
  gpus: Array<{
    name: string;
    memory_total_mb: number;
    memory_used_mb: number;
    memory_free_mb: number;
    utilization_percent: number;
    memory_utilization_percent: number;
  }>;
  defaults: {
    video_transcript_workers: number;
    video_compression_workers: number;
    pdf_render_workers: number;
    ocr_workers: number;
    zettel_read_workers: number;
    embedding_batch_size: number;
    embedding_workers: number;
  };
}

export interface WhatsAppContact {
  id: string;
  name: string;
  phone: string;
}

export interface Model {
  id: string;
  name: string;
}

export interface TokenUsage {
  input_tokens?: number;
  cached_input_tokens?: number;
  output_tokens?: number;
  reasoning_output_tokens?: number;
  total_tokens?: number;
}

export type ProgressMode = "determinate" | "indeterminate" | "timed";

export interface Job {
  id: string;
  type: string;
  status: string;
  stage: string;
  progress: number;
  current_step: number;
  total_steps: number;
  eta_seconds: number;
  progress_mode?: ProgressMode;
  completed_units?: number;
  total_units?: number;
  unit_label?: string;
  progress_started_at?: string;
  progress_ends_at?: string;
  input: string;
  output: string;
  error: string;
  created_at: string;
  updated_at: string;
  finished_at?: string;
}

export interface BrowserEntry {
  name: string;
  path: string;
  is_dir: boolean;
}

export interface UploadedFile {
  name: string;
  path: string;
  url: string;
}
