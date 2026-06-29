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

export interface TopperCopyQuestion {
  id: string;
  label: string;
  title?: string;
  answer_markdown: string;
  source_pages: number[];
  status: string;
  dimensions?: QuestionDimensions;
}

export interface TopperCopyReview {
  kind: "topper_copy_review";
  review_id: string;
  pdf_name: string;
  source_mode?: "pdf_direct" | "images" | string;
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
