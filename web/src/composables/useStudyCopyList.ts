import { computed, shallowRef } from "vue";
import { api } from "../lib/api";
import type { StudyCopyRecord } from "../types";
import { useToasts } from "./useToasts";

export function useStudyCopyList() {
  const toasts = useToasts();
  const status = shallowRef("Loading study copies...");
  const query = shallowRef("");
  const copies = shallowRef<StudyCopyRecord[]>([]);
  const loading = shallowRef(false);
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
    } catch (error) {
      status.value = error instanceof Error ? error.message : "Load failed";
      toasts.error("Study copies failed", status.value);
    } finally {
      loading.value = false;
    }
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

  return { status, query, copies, loading, importPath, importFolder, summary, loadCopies, importCopies };
}
