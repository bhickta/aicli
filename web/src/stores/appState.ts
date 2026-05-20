import { computed, reactive } from "vue";
import type { Model, Settings, SystemResources, ViewName } from "../types";
import { workflowDefinitions } from "../workflows/definitions";

export const appState = reactive({
  view: "chat" as ViewName,
  health: "checking",
  settings: null as Settings | null,
  systemResources: null as SystemResources | null,
  models: {} as Record<string, Model[]>,
  browserPath: "",
  workflow: {
    category: "Study",
    workflowId: "recall",
    pathValues: {} as Record<string, string>,
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
  appState.workflow.category = category;
  appState.workflow.workflowId = activeWorkflowDefinitions.value[0]?.id || "";
}
