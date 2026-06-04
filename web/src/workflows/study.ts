import type { WorkflowDefinition } from "../types";
import { documentPayload, providerModelField, providerModelPayload } from "./builders";

const topperCopyFields: WorkflowDefinition["fields"] = [
  providerModelField,
  { type: "path", id: "path", label: "Topper copy PDF" },
  { type: "number", id: "dpi", label: "Render DPI", min: 150, max: 400, default: 300 },
  { type: "number", id: "render_workers", label: "Render workers (0 = auto)", min: 0, default: 0 },
  { type: "number", id: "workers", label: "OCR workers (0 = auto)", min: 0, default: 0 },
];

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
  {
    id: "analyze",
    category: "Study",
    label: "Topper copy analysis",
    endpoint: "/api/workflows/analyze/run",
    fields: topperCopyFields,
    buildPayload: documentPayload,
  },
];
