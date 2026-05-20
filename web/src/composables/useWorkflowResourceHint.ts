import { computed } from "vue";
import { activeWorkflow, appState } from "../stores/appState";

export function useWorkflowResourceHint() {
  return computed(() => {
    const defaults = appState.systemResources?.defaults;
    if (!defaults) return "";

    if (appState.workflow.category === "Video" && activeWorkflow.value?.id === "video-course") {
      return `Auto workers: ${defaults.video_transcript_workers} transcribe, ${defaults.video_compression_workers} compress`;
    }
    if (appState.workflow.category === "Documents") {
      return `Auto workers: ${defaults.pdf_render_workers} render, ${defaults.ocr_workers} OCR`;
    }
    if (appState.workflow.category === "Zettel") {
      return `Auto workers: ${defaults.zettel_read_workers} note readers`;
    }
    return "";
  });
}
