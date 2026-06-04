import { computed, reactive, shallowRef } from "vue";
import { api, pollJob, parseJobOutput } from "../lib/api";
import type { Job, TopperCopyReview, TopperReviewRecord } from "../types";

export type TopperRerunAction = "ocr" | "questions" | "report" | "all";

export function useStudyArchive() {
  const status = shallowRef("Loading saved reviews...");
  const query = shallowRef("");
  const reviews = shallowRef<TopperReviewRecord[]>([]);
  const selectedRecord = shallowRef<TopperReviewRecord | null>(null);
  const selectedReview = shallowRef<TopperCopyReview | null>(null);
  const running = shallowRef(false);
  const deletePDF = shallowRef(false);
  const providerModel = reactive({ provider_id: "", model: "" });
  const questionWorkers = shallowRef(0);
  const ocrWorkers = shallowRef(0);

  const summary = computed(() => {
    if (!reviews.value.length) return "No saved reviews";
    return `${reviews.value.length} saved review(s)`;
  });
  const canRerunOCR = computed(() => {
    const review = selectedReview.value;
    return Boolean(review?.pages.length) && Boolean(review?.pages.every((page) => page.path || page.image_url));
  });

  async function loadReviews() {
    status.value = "Loading saved reviews...";
    const path = `/api/topper-reviews?query=${encodeURIComponent(query.value)}&limit=80`;
    const payload = await api<{ reviews: TopperReviewRecord[] }>(path);
    reviews.value = payload.reviews || [];
    status.value = "Loaded";
    if (!selectedRecord.value && reviews.value[0]) await openReview(reviews.value[0]);
  }

  async function openReview(record: TopperReviewRecord) {
    status.value = `Opening ${record.pdf_name || record.id}...`;
    const payload = await api<{ record: TopperReviewRecord; review: TopperCopyReview }>(`/api/topper-reviews/${encodeURIComponent(record.id)}`);
    selectedRecord.value = payload.record;
    selectedReview.value = payload.review;
    providerModel.provider_id = payload.record.provider_id;
    providerModel.model = payload.record.model;
    status.value = "Review loaded";
  }

  async function saveReview() {
    if (!selectedRecord.value || !selectedReview.value) return;
    status.value = "Saving review edits...";
    const payload = await api<{ review: TopperCopyReview }>(`/api/topper-reviews/${encodeURIComponent(selectedRecord.value.id)}`, {
      method: "PUT",
      body: JSON.stringify(selectedReview.value),
    });
    selectedReview.value = payload.review;
    status.value = "Saved";
    await loadReviews();
  }

  async function rerunReview(action: TopperRerunAction, pageNumbers: number[] = []) {
    if (!selectedRecord.value) return;
    running.value = true;
    status.value = "Starting rerun...";
    try {
      const payload = await api<{ job: Job }>(`/api/topper-reviews/${encodeURIComponent(selectedRecord.value.id)}/rerun`, {
        method: "POST",
        body: JSON.stringify({
          provider_id: providerModel.provider_id,
          model: providerModel.model,
          action,
          page_numbers: pageNumbers,
          question_split: true,
          question_workers: questionWorkers.value,
          workers: ocrWorkers.value,
        }),
      });
      const job = await pollJob(payload.job.id, (nextJob) => {
        status.value = nextJob.stage || `Running ${action}`;
      });
      if (job.status !== "completed") {
        status.value = job.error || `${action} failed`;
        return;
      }
      const output = parseJobOutput(job.output) as TopperCopyReview | null;
      if (output?.kind === "topper_copy_review") selectedReview.value = output;
      await loadReviews();
      if (selectedRecord.value) await openReview(selectedRecord.value);
      status.value = "Rerun complete";
    } finally {
      running.value = false;
    }
  }

  async function deleteReview() {
    if (!selectedRecord.value) return;
    const record = selectedRecord.value;
    status.value = `Deleting ${record.pdf_name || record.id}...`;
    await api(`/api/topper-reviews/${encodeURIComponent(record.id)}`, {
      method: "DELETE",
      body: JSON.stringify({ delete_pdf: deletePDF.value }),
    });
    selectedRecord.value = null;
    selectedReview.value = null;
    deletePDF.value = false;
    await loadReviews();
    status.value = "Deleted";
  }

  function updateSelectedReview(review: TopperCopyReview) {
    selectedReview.value = review;
  }

  return {
    status,
    query,
    reviews,
    selectedRecord,
    selectedReview,
    running,
    deletePDF,
    providerModel,
    questionWorkers,
    ocrWorkers,
    summary,
    canRerunOCR,
    loadReviews,
    openReview,
    saveReview,
    rerunReview,
    deleteReview,
    updateSelectedReview,
  };
}
