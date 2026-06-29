import type { Job } from "./core";

export interface StudyCopyRecord {
  id: string;
  source_path: string;
  source_hash: string;
  pdf_name: string;
  candidate_name: string;
  roll_no: string;
  email: string;
  test_code: string;
  paper: string;
  copy_date: string;
  page_count: number;
  question_count: number;
  unclear_count: number;
  status: string;
  render_status: string;
  ocr_status: string;
  question_status: string;
  analysis_status: string;
  report_status: string;
  last_error: string;
  metadata_json: string;
  created_at: string;
  updated_at: string;
}

export interface StudyPageRecord {
  copy_id: string;
  page_number: number;
  name: string;
  image_path: string;
  image_url: string;
  ocr_text: string;
  raw_ocr: string;
  layout_json: string;
  status: string;
  error: string;
  unclear_count: number;
  verified: boolean;
  created_at: string;
  updated_at: string;
}

export interface StudyQuestionRecord {
  id: string;
  copy_id: string;
  question_no: number;
  label: string;
  prompt_text: string;
  prompt_hi: string;
  marks: number;
  word_limit: number;
  answer_text: string;
  source_pages: number[];
  status: string;
  feedback_json: string;
  analysis_json: string;
  metadata_json: string;
  created_at: string;
  updated_at: string;
}

export interface StudyAnalysisRecord {
  id: string;
  copy_id: string;
  scope_type: string;
  scope_id: string;
  dimension_key: string;
  provider_id: string;
  model: string;
  result_json: string;
  created_at: string;
  updated_at: string;
}

export interface StudyCopyDetail {
  copy: StudyCopyRecord;
  pages: StudyPageRecord[];
  questions: StudyQuestionRecord[];
  analyses: StudyAnalysisRecord[];
}

export interface StudyBatchRecord {
  id: string;
  job_id: string;
  status: string;
  stage: string;
  provider_id: string;
  model: string;
  parallelism: number;
  force_rerun: boolean;
  total: number;
  completed: number;
  failed: number;
  started_at: string;
  finished_at: string;
  duration_ms: number;
  created_at: string;
  updated_at: string;
}

export interface StudyBatchResponse {
  batch: StudyBatchRecord;
  items?: StudyBatchItemRecord[];
  job?: Job;
}

export interface StudyBatchItemRecord {
  batch_id: string;
  copy_id: string;
  stage: string;
  status: string;
  error: string;
  error_kind: string;
  attempt: number;
  cache_hit: boolean;
  api_calls: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  started_at: string;
  finished_at: string;
  duration_ms: number;
  created_at: string;
  updated_at: string;
}
