import { shallowRef } from "vue";
import type {
  ApiCallUsage,
  InboxCandidatePreviewReport,
  InboxMergeReport,
  MetadataReport,
  TrainingExportReport,
} from "../../types";

export function useZettelReports() {
  const inboxReport = shallowRef<InboxMergeReport | null>(null);
  const metadataReport = shallowRef<MetadataReport | null>(null);
  const trainingReport = shallowRef<TrainingExportReport | null>(null);
  const candidatePreview = shallowRef<InboxCandidatePreviewReport | null>(null);
  const apiUsage = shallowRef<ApiCallUsage | null>(null);
  const rollbackJobID = shallowRef("");

  function resetReports() {
    candidatePreview.value = null;
    inboxReport.value = null;
    metadataReport.value = null;
    trainingReport.value = null;
  }

  function clearApiUsage() {
    apiUsage.value = null;
  }

  function updateApiUsage(output: unknown) {
    apiUsage.value = extractApiUsage(output);
  }

  return {
    inboxReport,
    metadataReport,
    trainingReport,
    candidatePreview,
    apiUsage,
    rollbackJobID,
    resetReports,
    clearApiUsage,
    updateApiUsage,
  };
}

function extractApiUsage(output: unknown): ApiCallUsage | null {
  if (!output || typeof output !== "object") return null;
  const root = output as { api_calls?: ApiCallUsage; proposal?: { api_calls?: ApiCallUsage } };
  return root.api_calls || root.proposal?.api_calls || null;
}
