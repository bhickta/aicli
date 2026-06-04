import type { WorkflowDefinition } from "../types";
import { providerModelField, providerModelPayload } from "./builders";

const topperCopyFields: WorkflowDefinition["fields"] = [
  { type: "stepProviderModel", id: "ocr", label: "OCR model" },
  { type: "stepProviderModel", id: "question", label: "Question split model" },
  { type: "stepProviderModel", id: "report", label: "Report model" },
  { type: "path", id: "path", label: "Topper copy PDF" },
  { type: "number", id: "dpi", label: "Render DPI", min: 150, max: 400, default: 300 },
  { type: "number", id: "render_workers", label: "Render workers (0 = auto)", min: 0, default: 0 },
  { type: "number", id: "workers", label: "OCR workers (0 = auto, capped by pages)", min: 0, default: 0 },
  { type: "checkbox", id: "question_split", label: "Question-wise split", checked: true },
  { type: "number", id: "question_workers", label: "Question split workers (0 = auto, capped by pages)", min: 0, default: 0 },
  { type: "checkbox", id: "unload_models", label: "Unload local models when job ends", checked: true },
];

function topperCopyPayload(values: Record<string, unknown>) {
  return {
    provider_id: values.ocr_provider_id,
    model: values.ocr_model,
    ocr_provider_id: values.ocr_provider_id,
    ocr_model: values.ocr_model,
    question_provider_id: values.question_provider_id,
    question_model: values.question_model,
    report_provider_id: values.report_provider_id,
    report_model: values.report_model,
    path: values.path,
    dpi: values.dpi,
    render_workers: values.render_workers,
    workers: values.workers,
    question_split: values.question_split !== false,
    question_workers: values.question_workers,
    unload_models: values.unload_models !== false,
  };
}

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
    buildPayload: topperCopyPayload,
  },
];
