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
