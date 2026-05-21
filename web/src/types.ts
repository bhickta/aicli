export type ViewName = "chat" | "workflows" | "zettel" | "jobs" | "providers" | "tools" | "settings";

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
  headers?: Record<string, string>;
}

export interface Settings {
  default_provider: string;
  default_model: string;
  providers: ProviderConfig[];
  tools: Record<string, string>;
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

export interface WorkflowField {
  type: "providerModel" | "text" | "datetime" | "textarea" | "select" | "number" | "checkbox" | "path" | "whatsappContact";
  id?: string;
  label?: string;
  rows?: number;
  placeholder?: string;
  value?: string;
  default?: string | number;
  min?: number;
  max?: number;
  checked?: boolean;
  picker?: "directory";
  options?: Array<{ value: string; label: string }>;
}

export interface WorkflowDefinition {
  id: string;
  category: string;
  label: string;
  endpoint: string;
  preferredProviderId?: string;
  preferredModel?: string;
  fields: WorkflowField[];
  buildPayload: (values: Record<string, unknown>) => Record<string, unknown>;
}

export interface InboxClaim {
  id: string;
  text: string;
  source?: string;
}

export interface InboxClaimLedger {
  claim_id: string;
  status: string;
  destination_path?: string;
  evidence?: string;
  reason?: string;
}

export interface InboxDestinationDiff {
  path: string;
  before?: string;
  after?: string;
  diff: string;
  created?: boolean;
}

export interface ProviderApiCallUsage {
  provider_id: string;
  total: number;
  chat: number;
  embeddings: number;
  vision: number;
  stream: number;
}

export interface ApiCallUsage {
  total: number;
  chat: number;
  embeddings: number;
  vision: number;
  stream: number;
  providers?: ProviderApiCallUsage[];
}

export interface InboxSourceResult {
  source_path: string;
  status: string;
  processed_path?: string;
  destination_paths?: string[];
  merged_count: number;
  deduped_count: number;
  pending_count: number;
  reason?: string;
  claims?: InboxClaim[];
  ledger?: InboxClaimLedger[];
  diffs?: InboxDestinationDiff[];
}

export interface InboxMergeReport {
  run_id: string;
  archive_path: string;
  processed?: InboxSourceResult[];
  pending?: InboxSourceResult[];
  failed?: InboxSourceResult[];
  source_count?: number;
  selected_count?: number;
  skipped_count?: number;
  limit?: number;
  processed_count: number;
  pending_count: number;
  failed_count: number;
  api_calls?: ApiCallUsage;
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
