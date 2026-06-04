import { computed, reactive, watch } from "vue";
import { readStoredRecord, readStoredString, readStoredValue, writeStoredJSON, writeStoredString } from "../lib/persistence";
import type { Model, Settings, SystemResources, ViewName, WhatsAppContact } from "../types";
import { workflowCategories, workflowDefinitions } from "../workflows/definitions";

const storedView = readStoredString("aicli.view", "chat") as ViewName;
const storedWorkflowCategory = normalizedWorkflowCategory(readStoredString("aicli.workflow.category", "Codex"));
const storedWorkflowID = readStoredString("aicli.workflow.id", "recall");

export const appState = reactive({
  view: storedView,
  health: "checking",
  settings: null as Settings | null,
  systemResources: null as SystemResources | null,
  models: {} as Record<string, Model[]>,
  browserPath: readStoredString("aicli.browserPath", ""),
  workflow: {
    category: storedWorkflowCategory,
    workflowId: storedWorkflowID,
    pathValues: readStoredRecord("aicli.workflow.pathValues") as Record<string, string>,
  },
  whatsapp: {
    contacts: readStoredValue<WhatsAppContact[]>("aicli.whatsapp.contacts", []),
  },
});

export const providers = computed(() => appState.settings?.providers || []);

export const defaultProviderId = computed(() => {
  return appState.settings?.default_provider || providers.value[0]?.id || "";
});

export const defaultModel = computed(() => appState.settings?.default_model || "");

export const activeWorkflowDefinitions = computed(() => {
  return workflowDefinitions.filter((workflow) => workflow.category === appState.workflow.category);
});

export const activeWorkflow = computed(() => {
  return activeWorkflowDefinitions.value.find((workflow) => workflow.id === appState.workflow.workflowId) || activeWorkflowDefinitions.value[0];
});

export function selectWorkflowCategory(category: string) {
  if (category === "Zettel") {
    appState.view = "zettel";
    return;
  }
  appState.workflow.category = category;
  appState.workflow.workflowId = activeWorkflowDefinitions.value[0]?.id || "";
}

function normalizedWorkflowCategory(category: string) {
  return workflowCategories.includes(category) ? category : workflowCategories[0] || "Codex";
}

watch(() => appState.view, (view) => writeStoredString("aicli.view", view));
watch(() => appState.browserPath, (path) => writeStoredString("aicli.browserPath", path));
watch(() => appState.workflow.category, (category) => writeStoredString("aicli.workflow.category", category));
watch(() => appState.workflow.workflowId, (workflowId) => writeStoredString("aicli.workflow.id", workflowId));
watch(() => appState.workflow.pathValues, (values) => writeStoredJSON("aicli.workflow.pathValues", values), { deep: true });
watch(() => appState.whatsapp.contacts, (contacts) => writeStoredJSON("aicli.whatsapp.contacts", contacts), { deep: true });
