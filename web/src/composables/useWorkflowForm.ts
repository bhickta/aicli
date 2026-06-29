import { computed, reactive, watch, type ComputedRef } from "vue";
import { readNumberValue } from "../lib/format";
import { readStoredRecord, writeStoredJSON } from "../lib/persistence";
import { appState } from "../stores/appState";
import type { WorkflowDefinition } from "../types";

export interface ProviderModelSelection {
  provider_id: string;
  model: string;
}

export function useWorkflowForm(activeWorkflow: ComputedRef<WorkflowDefinition | undefined>) {
  const values = reactive<Record<string, unknown>>({});
  const providerModel = reactive<ProviderModelSelection>({ provider_id: "", model: "" });

  const nonProviderFields = computed(() => activeWorkflow.value?.fields.filter((field) => field.type !== "providerModel") || []);
  const hasProviderModel = computed(() => activeWorkflow.value?.fields.some((field) => field.type === "providerModel") || false);

  watch(activeWorkflow, initializeValues, { immediate: true });
  watch(providerModel, saveProviderModel, { deep: true });

  function initializeValues() {
    for (const key of Object.keys(values)) delete values[key];
    const workflow = activeWorkflow.value;
    if (!workflow) return;
    const stored = readStoredRecord(workflowStorageKey(workflow.id));
    for (const field of workflow.fields) {
      if (!field.id) continue;
      if (field.type === "stepProviderModel") {
        values[field.id + "_provider_id"] = stored[field.id + "_provider_id"] || workflow.preferredProviderId || "";
        values[field.id + "_model"] = preferredModelValue(stored[field.id + "_model"], workflow.preferredModel);
        continue;
      }
      const storedValue = stored[field.id];
      if (storedValue !== undefined) {
        values[field.id] = storedValue;
      } else if (field.type === "path") {
        values[field.id] = appState.workflow.pathValues[field.id] || "";
      } else if (field.type === "checkbox") {
        values[field.id] = Boolean(field.checked);
      } else if (field.type === "number") {
        values[field.id] = field.default ?? 0;
      } else {
        values[field.id] = field.value ?? field.default ?? "";
      }
    }
    providerModel.provider_id = String(stored.provider_id || workflow.preferredProviderId || "");
    providerModel.model = preferredModelValue(stored.model, workflow.preferredModel);
  }

  function updateField(id: string, value: unknown) {
    values[id] = value;
    if (activeWorkflow.value?.fields.some((field) => field.id === id && field.type === "path")) {
      appState.workflow.pathValues[id] = typeof value === "string" ? value : appState.workflow.pathValues[id] || "";
    }
    saveWorkflowValues();
  }

  function collectValues() {
    const out: Record<string, unknown> = { ...values };
    if (hasProviderModel.value) {
      out.provider_id = providerModel.provider_id;
      out.model = providerModel.model;
    }
    for (const field of activeWorkflow.value?.fields || []) {
      if (!field.id || field.type !== "number") continue;
      out[field.id] = readNumberValue(out[field.id], Number(field.default || 0), Number(field.min || 0));
    }
    return out;
  }

  function saveWorkflowValues() {
    const workflow = activeWorkflow.value;
    if (!workflow) return;
    writeStoredJSON(workflowStorageKey(workflow.id), {
      ...values,
      provider_id: providerModel.provider_id,
      model: providerModel.model,
    });
  }

  function saveProviderModel() {
    if (!hasProviderModel.value) return;
    saveWorkflowValues();
  }

  return {
    values,
    providerModel,
    nonProviderFields,
    hasProviderModel,
    initializeValues,
    updateField,
    collectValues,
  };
}

function workflowStorageKey(workflowID: string) {
  return `aicli.workflow.${workflowID}.values`;
}

function preferredModelValue(stored: unknown, preferred = "") {
  const storedModel = typeof stored === "string" ? stored : "";
  if (isStaleFlashLiteModel(storedModel, preferred)) return preferred;
  return storedModel || preferred || "";
}

function isStaleFlashLiteModel(stored: string, preferred: string) {
  return Boolean(
    preferred &&
      stored &&
      stored !== preferred &&
      stored.toLowerCase().includes("flash-lite") &&
      preferred.toLowerCase().includes("flash-lite"),
  );
}
