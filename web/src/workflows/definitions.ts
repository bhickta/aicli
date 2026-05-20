import type { WorkflowDefinition } from "../types";
import { audioWorkflowDefinitions } from "./audio";
import { codexWorkflowDefinitions } from "./codex";
import { documentWorkflowDefinitions } from "./documents";
import { imageWorkflowDefinitions } from "./images";
import { newsWorkflowDefinitions } from "./news";
import { studyWorkflowDefinitions } from "./study";
import { videoWorkflowDefinitions } from "./video";

export const workflowCategories = ["Study", "Codex", "Documents", "Images", "Audio", "Video", "News"];

export const workflowDefinitions: WorkflowDefinition[] = [
  ...studyWorkflowDefinitions,
  ...codexWorkflowDefinitions,
  ...documentWorkflowDefinitions,
  ...imageWorkflowDefinitions,
  ...audioWorkflowDefinitions,
  ...videoWorkflowDefinitions,
  ...newsWorkflowDefinitions,
];
