export interface TopperCopyPage {
  number: number;
  name: string;
  path: string;
  image_url: string;
  text: string;
  unclear_count: number;
  verified: boolean;
  kind?: "answer" | "question_paper" | "cover" | "index" | "evaluation" | "blank" | "other" | "unknown" | string;
  kind_confidence?: number;
  classification_reason?: string;
  ocr_confidence?: number;
  ocr_issues?: string[];
}

export interface QuestionDimensions {
  introduction: string;
  outro: string;
  transition: string;
  diagram: string;
  fact: string;
  fact_usage: string;
  custom: string;
  demand_alignment?: string;
  body_structure?: string;
  content_depth?: string;
  multidimensionality?: string;
  presentation?: string;
  strengths?: AnalysisPoint[];
  gaps?: AnalysisPoint[];
  missing_dimensions?: string[];
  examiner_signals?: string[];
  improvements?: AnalysisImprovement[];
  reusable_techniques?: string[];
  scorecard?: QuestionScorecard;
}

export interface AnalysisPoint {
  point: string;
  evidence?: string;
  why_it_matters?: string;
}

export interface AnalysisImprovement {
  priority: "high" | "medium" | "low" | string;
  change: string;
  example?: string;
}

export interface QuestionScorecard {
  demand_fulfilment: number;
  structure: number;
  content_depth: number;
  evidence: number;
  multidimensionality: number;
  presentation: number;
  conclusion: number;
  overall_percent: number;
  estimated_band: string;
  confidence: string;
  rationale: string;
}

export interface AnalysisQuality {
  classification_coverage_percent: number;
  average_classification_confidence: number;
  ocr_assessment_coverage_percent: number;
  average_ocr_confidence: number;
  prompt_match_percent: number;
  analysis_coverage_percent: number;
  evidence_coverage_percent: number;
  ocr_unclear_percent: number;
  overall_coverage_percent: number;
  requires_review: boolean;
  warnings: string[];
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
  quality?: AnalysisQuality;
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
