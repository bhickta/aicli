import { computed } from "vue";
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

  async function startBatch(stage: string) {
    await runner.startBatch(selection.selectedIds.value, stage);
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
