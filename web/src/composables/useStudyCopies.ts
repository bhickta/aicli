import { computed, shallowRef } from "vue";
import { api } from "../lib/api";
import type { StudyBatchRecord, StudyCopyDetail, StudyCopyRecord } from "../types";
import { useToasts } from "./useToasts";

export function useStudyCopies() {
  const toasts = useToasts();
  const status = shallowRef("Loading study copies...");
  const query = shallowRef("");
  const copies = shallowRef<StudyCopyRecord[]>([]);
  const selected = shallowRef<StudyCopyDetail | null>(null);
  const loading = shallowRef(false);
  const selectedIds = shallowRef<string[]>([]);
  const importPath = shallowRef("");
  const importFolder = shallowRef("");

  const summary = computed(() => {
    const pageCount = copies.value.reduce((total, copy) => total + copy.page_count, 0);
    const questionCount = copies.value.reduce((total, copy) => total + copy.question_count, 0);
    return `${copies.value.length} copies, ${pageCount} pages, ${questionCount} questions`;
  });

  async function loadCopies() {
    loading.value = true;
    status.value = "Loading study copies...";
    try {
      const payload = await api<{ copies: StudyCopyRecord[] }>(
        `/api/study/copies?query=${encodeURIComponent(query.value)}&limit=200`,
      );
      copies.value = payload.copies || [];
      status.value = "Loaded";
      if (!selected.value && copies.value[0]) await openCopy(copies.value[0].id);
    } catch (error) {
      status.value = error instanceof Error ? error.message : "Load failed";
      toasts.error("Study copies failed", status.value);
    } finally {
      loading.value = false;
    }
  }

  async function openCopy(id: string) {
    status.value = "Opening copy...";
    selected.value = await api<StudyCopyDetail>(`/api/study/copies/${encodeURIComponent(id)}`);
    status.value = "Copy loaded";
  }

  async function importCopies() {
    const paths = importPath.value
      .split(/\n|,/)
      .map((path) => path.trim())
      .filter(Boolean);
    if (!paths.length && !importFolder.value.trim()) return;
    status.value = "Importing PDFs...";
    const payload = await api<{ copies: StudyCopyRecord[] }>("/api/study/imports", {
      method: "POST",
      body: JSON.stringify({ paths, folder_path: importFolder.value.trim(), recursive: true }),
    });
    toasts.success("Imported study copies", `${payload.copies.length} PDF(s) tracked`);
    importPath.value = "";
    importFolder.value = "";
    await loadCopies();
  }

  async function startBatch(stage: string) {
    if (!selectedIds.value.length) return;
    const payload = await api<{ batch: StudyBatchRecord }>("/api/study/batches", {
      method: "POST",
      body: JSON.stringify({ copy_ids: selectedIds.value, stage }),
    });
    toasts.info("Batch queued", `${payload.batch.total} copy(s), ${payload.batch.stage}`);
  }

  function toggleCopy(id: string) {
    selectedIds.value = selectedIds.value.includes(id)
      ? selectedIds.value.filter((item) => item !== id)
      : [...selectedIds.value, id];
  }

  return {
    status,
    query,
    copies,
    selected,
    loading,
    selectedIds,
    importPath,
    importFolder,
    summary,
    loadCopies,
    openCopy,
    importCopies,
    startBatch,
    toggleCopy,
  };
}
