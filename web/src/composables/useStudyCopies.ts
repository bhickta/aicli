import { computed, watch } from "vue";
import type { StudyBatchItemRecord, StudyCopyRecord } from "../types";
import { useStudyBatchRunner } from "./useStudyBatchRunner";
import { useStudyCopyList } from "./useStudyCopyList";
import { useStudyCopySelection } from "./useStudyCopySelection";

export function useStudyCopies() {
  const list = useStudyCopyList();
  const selection = useStudyCopySelection();
  const runner = useStudyBatchRunner(async () => {
    await list.loadCopies();
    const id = selection.selected.value?.copy.id;
    if (id) await selection.openCopy(id);
  });

  const status = computed(() => runner.runStatus.value || selection.selectionStatus.value || list.status.value);
  const refreshedItems = new Set<string>();

  watch(
    () => runner.batchItems.value,
    (items) => {
      applyBatchItems(items);
      void refreshSettledSelection(items);
    },
  );

  async function startBatch(stage: string) {
    await runner.startBatch(selection.selectedIds.value, stage);
  }

  function applyBatchItems(items: StudyBatchItemRecord[]) {
    if (!items.length) return;
    const byCopy = new Map(items.map((item) => [item.copy_id, item]));
    list.copies.value = list.copies.value.map((copy) => {
      const item = byCopy.get(copy.id);
      return item ? copyWithRunState(copy, item) : copy;
    });
    const detail = selection.selected.value;
    if (detail) {
      const item = byCopy.get(detail.copy.id);
      if (item) selection.selected.value = { ...detail, copy: copyWithRunState(detail.copy, item) };
    }
  }

  async function refreshSettledSelection(items: StudyBatchItemRecord[]) {
    const selectedID = selection.selected.value?.copy.id;
    if (!selectedID) return;
    const item = items.find((candidate) => candidate.copy_id === selectedID);
    if (!item || !["ready", "failed"].includes(item.status)) return;
    const key = `${item.batch_id}:${item.copy_id}:${item.status}`;
    if (refreshedItems.has(key)) return;
    refreshedItems.add(key);
    await selection.openCopy(selectedID);
    await list.loadCopies();
  }

  function copyWithRunState(copy: StudyCopyRecord, item: StudyBatchItemRecord): StudyCopyRecord {
    if (item.status === "ready") return { ...copy, status: "ready", last_error: "" };
    if (item.status === "failed") return { ...copy, status: "failed", last_error: item.error || "Analysis failed" };
    return { ...copy, status: "running", analysis_status: "running", last_error: "" };
  }

  return {
    status,
    query: list.query,
    copies: list.copies,
    selected: selection.selected,
    loading: list.loading,
    selectedIds: selection.selectedIds,
    importPath: list.importPath,
    importFolder: list.importFolder,
    batchParallelism: runner.batchParallelism,
    forceRerun: runner.forceRerun,
    running: runner.running,
    runStatus: runner.runStatus,
    activeBatch: runner.activeBatch,
    batchItems: runner.batchItems,
    summary: list.summary,
    loadCopies: list.loadCopies,
    openCopy: selection.openCopy,
    clearSelectedCopy: selection.clearSelectedCopy,
    importCopies: list.importCopies,
    startBatch,
    runCopy: runner.runCopy,
    toggleCopy: selection.toggleCopy,
  };
}
