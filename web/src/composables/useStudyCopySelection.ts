import { shallowRef } from "vue";
import { api } from "../lib/api";
import type { StudyCopyDetail } from "../types";

export function useStudyCopySelection() {
  const selected = shallowRef<StudyCopyDetail | null>(null);
  const selectedIds = shallowRef<string[]>([]);
  const selectionStatus = shallowRef("");

  async function openCopy(id: string) {
    selectionStatus.value = "Opening copy...";
    selected.value = await api<StudyCopyDetail>(`/api/study/copies/${encodeURIComponent(id)}`);
    selectionStatus.value = "Copy loaded";
  }

  function clearSelectedCopy() {
    selected.value = null;
  }

  function toggleCopy(id: string) {
    selectedIds.value = selectedIds.value.includes(id)
      ? selectedIds.value.filter((item) => item !== id)
      : [...selectedIds.value, id];
  }

  return { selected, selectedIds, selectionStatus, openCopy, clearSelectedCopy, toggleCopy };
}
