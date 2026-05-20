import { computed, reactive, watch, type ComputedRef } from "vue";
import { readNumberValue } from "../lib/format";
import { appState } from "../stores/appState";
import type { WorkflowDefinition } from "../types";

interface ProviderModelSelection {
  provider_id: string;
  model: string;
}

export function useWorkflowForm(activeWorkflow: ComputedRef<WorkflowDefinition | undefined>) {
  const values = reactive<Record<string, unknown>>({});
  const providerModel = reactive<ProviderModelSelection>({ provider_id: "", model: "" });

  const nonProviderFields = computed(() => activeWorkflow.value?.fields.filter((field) => field.type !== "providerModel") || []);
  const hasProviderModel = computed(() => activeWorkflow.value?.fields.some((field) => field.type === "providerModel") || false);

  watch(activeWorkflow, initializeValues, { immediate: true });

  function initializeValues() {
    for (const key of Object.keys(values)) delete values[key];
    const workflow = activeWorkflow.value;
    if (!workflow) return;
    for (const field of workflow.fields) {
      if (!field.id) continue;
      if (field.type === "path") values[field.id] = appState.workflow.pathValues[field.id] || "";
      else if (field.type === "checkbox") values[field.id] = Boolean(field.checked);
      else if (field.type === "number") values[field.id] = field.default ?? 0;
      else values[field.id] = field.value ?? field.default ?? "";
    }
  }

  function updateField(id: string, value: unknown) {
    values[id] = value;
    appState.workflow.pathValues[id] = typeof value === "string" ? value : appState.workflow.pathValues[id] || "";
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
