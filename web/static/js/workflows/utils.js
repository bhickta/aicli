import { state, WORKFLOW_PREFIX } from "../core/state.js";
import { WORKFLOW_CATEGORIES, WORKFLOW_DEFINITIONS } from "./definitions/index.js";

export function workflowControlId(id) {
  return `${WORKFLOW_PREFIX}-${id}`;
}

export function workflowDisplayId(id) {
  return `${WORKFLOW_PREFIX}-${id}-value`;
}

export function workflowBrowseId(id) {
  return `${WORKFLOW_PREFIX}-${id}-browse`;
}

export function activeWorkflowCategoryDefinitions(category) {
  if (category === "Video") {
    return WORKFLOW_DEFINITIONS.filter((workflow) => workflow.id === "video-course");
  }
  return WORKFLOW_DEFINITIONS.filter((workflow) => workflow.category === category);
}

export function getWorkflowById(workflowId) {
  return WORKFLOW_DEFINITIONS.find((workflow) => workflow.id === workflowId) || null;
}

export function getActiveWorkflow() {
  const byCategory = activeWorkflowCategoryDefinitions(state.workflow.category);
  if (!byCategory.length) {
    state.workflow.category = WORKFLOW_CATEGORIES[0];
    state.workflow.workflowId = WORKFLOW_DEFINITIONS[0].id;
    return WORKFLOW_DEFINITIONS[0];
  }
  const match = getWorkflowById(state.workflow.workflowId);
  if (match && byCategory.some((workflow) => workflow.id === match.id)) {
    return match;
  }
  state.workflow.workflowId = byCategory[0].id;
  return byCategory[0];
}
