import { shallowRef, type Ref } from "vue";
import { api } from "../lib/api";
import { appState } from "../stores/appState";
import type { WorkflowField } from "../types";

interface WorkflowBrowserOptions {
  values: Record<string, unknown>;
  updateField: (id: string, value: unknown) => void;
  status: Ref<string>;
  result: Ref<string>;
}

export function useWorkflowBrowser(options: WorkflowBrowserOptions) {
  const browserField = shallowRef<WorkflowField | null>(null);

  async function browse(field: WorkflowField) {
    if (field.picker === "directory" && field.id) {
      await pickSystemDirectory(field);
      return;
    }
    if (field.id && await pickSystemFile(field)) return;
    browserField.value = field;
  }

  async function pickSystemDirectory(field: WorkflowField) {
    if (!field.id) return;

    options.status.value = `Opening system folder picker for ${field.label}...`;
    try {
      const currentPath = String(options.values[field.id] || appState.browserPath || "");
      const query = currentPath ? `?path=${encodeURIComponent(currentPath)}` : "";
      const picked = await api<{ path: string }>(`/api/fs/pick-directory${query}`);
      options.updateField(field.id, picked.path);
      appState.browserPath = picked.path;
      options.status.value = `Selected ${field.label}`;
      if (options.result.value.includes("For in-place processing")) {
        options.result.value = "";
      }
    } catch (error) {
      options.status.value = "Folder picker failed";
      options.result.value = error instanceof Error ? error.message : "Folder picker failed";
    }
  }

  async function pickSystemFile(field: WorkflowField) {
    if (!field.id) return false;

    options.status.value = `Opening system file picker for ${field.label}...`;
    try {
      const currentPath = String(options.values[field.id] || appState.browserPath || "");
      const query = currentPath ? `?path=${encodeURIComponent(currentPath)}` : "";
      const picked = await api<{ path: string }>(`/api/fs/pick-file${query}`);
      options.updateField(field.id, picked.path);
      appState.browserPath = picked.path;
      options.status.value = `Selected ${field.label}`;
      return true;
    } catch (error) {
      options.status.value = "Using app file browser";
      options.result.value = error instanceof Error ? error.message : "System file picker unavailable";
      return false;
    }
  }

  function handleBrowserSelect(id: string, path: string) {
    options.updateField(id, path);
    browserField.value = null;
  }

  function closeBrowser() {
    browserField.value = null;
  }

  return {
    browserField,
    browse,
    handleBrowserSelect,
    closeBrowser,
  };
}
