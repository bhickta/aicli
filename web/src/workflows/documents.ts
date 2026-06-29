import type { WorkflowDefinition } from "../types";
import { documentPayload, providerModelField } from "./builders";

const documentFields: WorkflowDefinition["fields"] = [
  providerModelField,
  { type: "path", id: "path", label: "Input PDF file" },
  { type: "number", id: "dpi", label: "Render DPI", min: 100, max: 400, default: 200 },
  { type: "number", id: "render_workers", label: "Render workers (0 = auto)", min: 0, default: 0 },
  { type: "number", id: "workers", label: "OCR workers (local models capped to 1)", min: 0, default: 0 },
];

export const documentWorkflowDefinitions: WorkflowDefinition[] = [
  {
    id: "ocr-run",
    category: "Documents",
    label: "OCR: ZIP images to Markdown",
    endpoint: "/api/workflows/ocr/run",
    fields: [
      providerModelField,
      { type: "path", id: "path", label: "Input ZIP file" },
      { type: "number", id: "render_workers", label: "Render workers (0 = auto)", min: 0, default: 0 },
      { type: "number", id: "workers", label: "OCR workers (local models capped to 1)", min: 0, default: 0 },
    ],
    buildPayload: documentPayload,
  },
  {
    id: "ocr-pdf",
    category: "Documents",
    label: "OCR: PDF to Markdown",
    endpoint: "/api/workflows/ocr/pdf",
    fields: documentFields,
    buildPayload: documentPayload,
  },
];
