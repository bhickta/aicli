import type { WorkflowDefinition } from "../types";
import { audioWorkflowDefinitions } from "./audio";
import { codexWorkflowDefinitions } from "./codex";
import { documentWorkflowDefinitions } from "./documents";
import { imageWorkflowDefinitions } from "./images";
import { newsWorkflowDefinitions } from "./news";
import { studyWorkflowDefinitions } from "./study";
import { videoWorkflowDefinitions } from "./video";
import { whatsappWorkflowDefinitions } from "./whatsapp";

export const workflowCategories = ["Study", "Zettel", "Codex", "Documents", "Images", "Audio", "Video", "WhatsApp", "News"];

export const workflowDefinitions: WorkflowDefinition[] = [
  ...studyWorkflowDefinitions,
  ...codexWorkflowDefinitions,
  ...documentWorkflowDefinitions,
  ...imageWorkflowDefinitions,
  ...audioWorkflowDefinitions,
  ...videoWorkflowDefinitions,
  ...whatsappWorkflowDefinitions,
  ...newsWorkflowDefinitions,
];
