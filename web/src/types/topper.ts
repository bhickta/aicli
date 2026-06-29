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
