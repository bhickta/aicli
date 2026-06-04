export type ViewName = "chat" | "workflows" | "study-archive" | "zettel" | "jobs" | "providers" | "tools" | "settings";

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

export interface WorkflowField {
  type: "providerModel" | "stepProviderModel" | "text" | "datetime" | "textarea" | "select" | "number" | "checkbox" | "path" | "whatsappContact";
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

export interface TopperCopyPage {
  number: number;
  name: string;
  path: string;
  image_url: string;
  text: string;
  unclear_count: number;
  verified: boolean;
}

export interface TopperCopyQuestion {
  id: string;
  label: string;
  title?: string;
  answer_markdown: string;
  source_pages: number[];
  status: string;
}

export interface TopperCopyReview {
  kind: "topper_copy_review";
  review_id: string;
  pdf_name: string;
  pages: TopperCopyPage[];
  questions: TopperCopyQuestion[];
  report: string;
}

export interface TopperReviewRecord {
  id: string;
  job_id: string;
  pdf_name: string;
  source_path: string;
  provider_id: string;
  model: string;
  page_count: number;
  question_count: number;
  unclear_count: number;
  status: string;
  created_at: string;
  updated_at: string;
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

export interface InboxCandidate {
  path: string;
  similarity: number;
  excerpt?: string;
}

export interface InboxCandidateSource {
  source_path: string;
  source_excerpt?: string;
  candidates: InboxCandidate[];
  error?: string;
}

export interface InboxCandidatePreviewReport {
  sources: InboxCandidateSource[];
  source_count: number;
  selected_count: number;
  skipped_count: number;
  limit?: number;
  api_calls?: ApiCallUsage;
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
  source_content?: string;
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

export interface MetadataNoteResult {
  path: string;
  status: string;
  reason?: string;
  title?: string;
  summary_keywords?: string;
  recall_questions?: string[];
  diff?: InboxDestinationDiff;
}

export interface MetadataReport {
  run_id: string;
  archive_path: string;
  processed?: MetadataNoteResult[];
  skipped?: MetadataNoteResult[];
  failed?: MetadataNoteResult[];
  source_count?: number;
  selected_count?: number;
  skipped_count?: number;
  limit?: number;
  processed_count: number;
  failed_count: number;
  api_calls?: ApiCallUsage;
}

export interface TrainingExportReport {
  run_id: string;
  archive_path: string;
  train_path: string;
  eval_path: string;
  sharegpt_train_path?: string;
  sharegpt_eval_path?: string;
  manifest_path: string;
  source_files?: string[];
  strict?: boolean;
  scanned_count: number;
  exported_count: number;
  train_count: number;
  eval_count: number;
  duplicate_count: number;
  skipped_count: number;
  skipped_by_reason?: Record<string, number>;
  quality?: TrainingQualityReport;
  api_calls?: ApiCallUsage;
}

export interface TrainingQualityReport {
  system_prompt_variants: number;
  primary_system_prompt_count: number;
  examples_with_semantic_candidates: number;
  examples_without_semantic_candidates: number;
  examples_with_code_fences: number;
  examples_with_duplicate_frontmatter: number;
  examples_with_bad_note_boundaries: number;
  examples_with_status_or_json_output: number;
  short_assistant_count: number;
  long_assistant_count: number;
  total_final_notes: number;
  max_final_notes_per_example: number;
  min_user_chars: number;
  max_user_chars: number;
  average_user_chars: number;
  min_assistant_chars: number;
  max_assistant_chars: number;
  average_assistant_chars: number;
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
