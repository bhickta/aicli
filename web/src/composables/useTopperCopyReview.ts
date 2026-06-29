import { computed, reactive, shallowRef, watch, type ComputedRef } from "vue";
import type { TopperCopyPage, TopperCopyQuestion, TopperCopyReview } from "../types";

export type TopperRerunAction = "ocr" | "questions" | "analytics" | "report" | "all";

interface TopperReviewProps {
  review: TopperCopyReview;
  editable?: boolean;
}

export function useTopperCopyReview(
  props: TopperReviewProps,
  emitReviewUpdate: (review: TopperCopyReview) => void,
  emitRerunPage: (action: TopperRerunAction, pageNumber: number) => void,
) {
  const activeTab = shallowRef<"pages" | "questions" | "report">(initialTab(props.review));
  const activePageNumber = shallowRef(props.review.pages[0]?.number || 1);
  const activeQuestionID = shallowRef(props.review.questions[0]?.id || "");
  const zoom = shallowRef(1);
  const fullscreen = shallowRef(false);
  const pageEdits = reactive<Record<number, { text: string; verified: boolean }>>({});
  const questionEdits = reactive<Record<string, { answer: string; title: string; status: string }>>({});
  const reportEdit = shallowRef(props.review.report);

  watch(() => props.review, syncEdits, { immediate: true });

  const activePage = computed<TopperCopyPage | undefined>(() =>
    props.review.pages.find((page) => page.number === activePageNumber.value) || props.review.pages[0],
  );
  const activePageEdit = computed(() => {
    if (!activePage.value) return null;
    return pageEdits[activePage.value.number] || { text: activePage.value.text, verified: activePage.value.verified };
  });
  const activeQuestion = computed<TopperCopyQuestion | undefined>(() =>
    props.review.questions.find((question) => question.id === activeQuestionID.value) || props.review.questions[0],
  );
  const activeQuestionEdit = computed(() => {
    if (!activeQuestion.value) return null;
    return questionEdits[activeQuestion.value.id] || {
      answer: activeQuestion.value.answer_markdown,
      title: activeQuestion.value.title || "",
      status: activeQuestion.value.status,
    };
  });
  const verifiedCount = computed(() => props.review.pages.filter((page) => pageEdits[page.number]?.verified).length);
  const unclearCount = computed(() => props.review.pages.reduce((total, page) => total + page.unclear_count, 0));
  const sourcePageLabels = computed(() => activeQuestion.value?.source_pages.map((page) => `P${page}`).join(", ") || "");
  const activePageHasImage = computed(() => Boolean(activePage.value?.image_url || activePage.value?.path));
  const isPDFDirect = computed(() => props.review.source_mode === "pdf_direct");
  const hasQuestionBlocks = computed(() => isPDFDirect.value || props.review.questions.some((question) => !isPageFallbackQuestion(question)));
  const questionTabLabel = computed(() => props.review.questions.length > 0 ? `Question-wise (${props.review.questions.length})` : "Question-wise");
  const pageTabLabel = computed(() => isPDFDirect.value ? "Source Pages" : "Page OCR");
  const pagePanelTitle = computed(() => isPDFDirect.value ? "Source text" : "OCR text");
  const pagePanelEmptyText = computed(() => isPDFDirect.value ? "No source notes were saved for this PDF page." : "No OCR text for this page.");

  function syncEdits(review: TopperCopyReview, previousReview?: TopperCopyReview) {
    const previousPageNumber = activePageNumber.value;
    const previousQuestionID = activeQuestionID.value;
    const reviewChanged = review.review_id !== previousReview?.review_id;
    for (const page of review.pages) pageEdits[page.number] = { text: page.text, verified: page.verified };
    for (const question of review.questions) {
      questionEdits[question.id] = { answer: question.answer_markdown, title: question.title || "", status: question.status };
    }
    reportEdit.value = review.report;
    activePageNumber.value = pickActivePage(review, reviewChanged, previousPageNumber);
    activeQuestionID.value = pickActiveQuestion(review, reviewChanged, previousQuestionID);
    if (reviewChanged) activeTab.value = initialTab(review);
  }

  function emitReview() {
    if (!props.editable) return;
    emitReviewUpdate({
      ...props.review,
      pages: props.review.pages.map((page) => ({ ...page, text: pageEdits[page.number]?.text ?? page.text, verified: pageEdits[page.number]?.verified ?? page.verified })),
      questions: props.review.questions.map((question) => ({
        ...question,
        title: questionEdits[question.id]?.title ?? question.title,
        status: questionEdits[question.id]?.status ?? question.status,
        answer_markdown: questionEdits[question.id]?.answer ?? question.answer_markdown,
      })),
      report: reportEdit.value,
    });
  }

  return {
    activeTab,
    activePageNumber,
    activeQuestionID,
    zoom,
    fullscreen,
    pageEdits,
    questionEdits,
    reportEdit,
    activePage,
    activePageEdit,
    activeQuestion,
    activeQuestionEdit,
    verifiedCount,
    unclearCount,
    sourcePageLabels,
    activePageHasImage,
    isPDFDirect,
    hasQuestionBlocks,
    questionTabLabel,
    pageTabLabel,
    pagePanelTitle,
    pagePanelEmptyText,
    selectPage: (pageNumber: number) => {
      activeTab.value = "pages";
      activePageNumber.value = pageNumber;
    },
    selectQuestion: (question: TopperCopyQuestion) => {
      activeTab.value = "questions";
      activeQuestionID.value = question.id;
      if (question.source_pages[0]) activePageNumber.value = question.source_pages[0];
    },
    setZoom: (nextZoom: number) => {
      zoom.value = Math.min(2, Math.max(0.6, nextZoom));
    },
    updateActivePageText: updatePageText(activePage, pageEdits, emitReview),
    toggleActivePageVerified: togglePageVerified(activePage, activePageEdit, pageEdits, emitReview),
    updateActiveQuestionAnswer: updateQuestionField(activeQuestion, activeQuestionEdit, questionEdits, "answer", emitReview),
    updateActiveQuestionTitle: updateQuestionField(activeQuestion, activeQuestionEdit, questionEdits, "title", emitReview),
    updateActiveQuestionStatus: updateQuestionField(activeQuestion, activeQuestionEdit, questionEdits, "status", emitReview),
    updateReport: (value: string) => {
      reportEdit.value = value;
      emitReview();
    },
    rerunActivePage: (action: TopperRerunAction) => {
      if (activePage.value) emitRerunPage(action, activePage.value.number);
    },
  };
}

function initialTab(review: TopperCopyReview): "pages" | "questions" | "report" {
  return review.source_mode === "pdf_direct" && review.questions.length > 0 ? "questions" : "pages";
}

function pickActivePage(review: TopperCopyReview, changed: boolean, previous: number) {
  if (changed) return review.pages[0]?.number || 1;
  return review.pages.some((page) => page.number === previous) ? previous : review.pages[0]?.number || 1;
}

function pickActiveQuestion(review: TopperCopyReview, changed: boolean, previous: string) {
  if (changed) return review.questions[0]?.id || "";
  return review.questions.some((question) => question.id === previous) ? previous : review.questions[0]?.id || "";
}

function isPageFallbackQuestion(question: TopperCopyQuestion) {
  return question.status === "needs review" && /^Page \d+( OCR)?$/i.test(question.label);
}

function updatePageText(activePage: ComputedRef<TopperCopyPage | undefined>, pageEdits: Record<number, { text: string; verified: boolean }>, emitReview: () => void) {
  return (value: string) => {
    if (!activePage.value) return;
    pageEdits[activePage.value.number] = { text: value, verified: pageEdits[activePage.value.number]?.verified || false };
    emitReview();
  };
}

function togglePageVerified(activePage: ComputedRef<TopperCopyPage | undefined>, activePageEdit: ComputedRef<{ text: string; verified: boolean } | null>, pageEdits: Record<number, { text: string; verified: boolean }>, emitReview: () => void) {
  return () => {
    if (!activePage.value) return;
    pageEdits[activePage.value.number] = { text: activePageEdit.value?.text || activePage.value.text, verified: !activePageEdit.value?.verified };
    emitReview();
  };
}

function updateQuestionField(activeQuestion: ComputedRef<TopperCopyQuestion | undefined>, activeQuestionEdit: ComputedRef<{ answer: string; title: string; status: string } | null>, questionEdits: Record<string, { answer: string; title: string; status: string }>, field: "answer" | "title" | "status", emitReview: () => void) {
  return (value: string) => {
    if (!activeQuestion.value) return;
    const current = activeQuestionEdit.value;
    questionEdits[activeQuestion.value.id] = {
      answer: field === "answer" ? value : current?.answer || activeQuestion.value.answer_markdown,
      title: field === "title" ? value : current?.title || activeQuestion.value.title || "",
      status: field === "status" ? value : current?.status || activeQuestion.value.status,
    };
    emitReview();
  };
}
