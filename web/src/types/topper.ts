export interface TopperCopyPage {
  number: number;
  name: string;
  path: string;
  image_url: string;
  text: string;
  unclear_count: number;
  verified: boolean;
}

export interface QuestionDimensions {
  introduction: string;
  outro: string;
  transition: string;
  diagram: string;
  fact: string;
  fact_usage: string;
  custom: string;
}

export interface CopyMetadata {
  suggested_pdf_name?: string;
  topper_name?: string;
  candidate_name?: string;
  rank?: string;
  exam?: string;
  year?: string;
  paper?: string;
  subject?: string;
  test_series?: string;
  coaching_institute?: string;
  test_code?: string;
  test_date?: string;
  language?: string;
  tags?: string[];
  search_hints?: string[];
  notes?: string;
}

export interface QuestionMetadata {
  subject?: string;
  topic?: string;
  subtopic?: string;
  syllabus_area?: string;
  paper?: string;
  question_type?: string;
  demand?: string;
  difficulty?: string;
  marks?: number;
  word_limit?: number;
  tags?: string[];
  search_hints?: string[];
}

export interface TopperCopyQuestion {
  id: string;
  label: string;
  title?: string;
  answer_markdown: string;
  source_pages: number[];
  status: string;
  dimensions?: QuestionDimensions;
  metadata?: QuestionMetadata;
}

export interface TopperCopyReview {
  kind: "topper_copy_review";
  review_id: string;
  pdf_name: string;
  source_mode?: "pdf_direct" | "images" | string;
  metadata?: CopyMetadata;
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
