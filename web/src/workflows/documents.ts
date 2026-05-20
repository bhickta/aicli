import type { WorkflowDefinition } from "../types";
import { documentPayload, providerModelField } from "./builders";

const documentFields: WorkflowDefinition["fields"] = [
  providerModelField,
  { type: "path", id: "path", label: "Input PDF file" },
  { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
  { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
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
      { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
      { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
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
  {
    id: "analyze",
    category: "Documents",
    label: "Analyze: PDF report",
    endpoint: "/api/workflows/analyze/run",
    fields: documentFields,
    buildPayload: documentPayload,
  },
];
