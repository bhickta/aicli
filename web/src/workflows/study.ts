import type { WorkflowDefinition } from "../types";
import { providerModelField, providerModelPayload } from "./builders";

export const studyWorkflowDefinitions: WorkflowDefinition[] = [
  {
    id: "recall",
    category: "Study",
    label: "Recall",
    endpoint: "/api/workflows/recall/run",
    fields: [
      providerModelField,
      { type: "textarea", id: "notes", label: "Notes", rows: 12, placeholder: "Paste UPSC notes..." },
    ],
    buildPayload: (values) => ({
      ...providerModelPayload(values),
      notes: values.notes,
    }),
  },
];
