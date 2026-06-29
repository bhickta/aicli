import type { WorkflowDefinition } from "../types";
import { providerModelField, providerModelPayload } from "./builders";

const topperCopyFields: WorkflowDefinition["fields"] = [
  { type: "stepProviderModel", id: "ocr", label: "OCR vision model (e.g. unlimited-ocr)" },
  { type: "stepProviderModel", id: "question", label: "Question split model" },
  { type: "stepProviderModel", id: "report", label: "Report model" },
  { type: "path", id: "path", label: "Topper copy PDF" },
  { type: "number", id: "dpi", label: "Render DPI", min: 150, max: 400, default: 300 },
  { type: "number", id: "render_workers", label: "Render workers (0 = auto)", min: 0, default: 0 },
  { type: "number", id: "workers", label: "OCR workers (0 = safe auto, explicit = parallel)", min: 0, default: 0 },
  { type: "number", id: "ocr_batch_size", label: "OCR pages per request (0 = auto, Gemini 5, local 1)", min: 0, max: 10, default: 0 },
  { type: "checkbox", id: "force_ocr", label: "Rerun OCR even if saved pages exist", checked: false },
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
    ocr_batch_size: values.ocr_batch_size,
    force_ocr: values.force_ocr === true,
    question_split: values.question_split !== false,
    question_workers: values.question_workers,
    unload_models: values.unload_models !== false,
  };
}

const lectureFields: WorkflowDefinition["fields"] = [
  providerModelField,
  { type: "path", id: "vault_path", label: "Obsidian vault", picker: "directory", default: "/home/bhickta/development/upsc" },
  { type: "path", id: "source_path", label: "Notes folder or note", picker: "directory", default: "/home/bhickta/development/upsc/zettelkasten" },
  { type: "text", id: "output_name", label: "Lecture title", placeholder: "Optional, generated from note/folder if empty" },
  {
    type: "select",
    id: "style",
    label: "Lecture style",
    default: "crisp comprehensive UPSC lecture",
    options: [
      { value: "crisp comprehensive UPSC lecture", label: "Crisp comprehensive" },
      { value: "Hinglish UPSC classroom lecture with English technical terms", label: "Hinglish classroom" },
      { value: "exam-focused revision lecture with examples and recall hooks", label: "Revision lecture" },
    ],
  },
  { type: "number", id: "max_notes", label: "Max notes", min: 1, default: 25 },
  { type: "number", id: "max_input_chars", label: "Max input characters", min: 4000, default: 120000 },
  { type: "checkbox", id: "synthesize_audio", label: "Generate audio with ots.TTS SOAR", checked: true },
  { type: "text", id: "tts_command", label: "TTS command", default: "ots.TTS" },
  { type: "text", id: "tts_args", label: "TTS args", default: 'SOAR --input "{script}" --output "{audio}"' },
];

function lecturePayload(values: Record<string, unknown>) {
  return {
    ...providerModelPayload(values),
    vault_path: values.vault_path,
    source_path: values.source_path,
    output_name: values.output_name,
    style: values.style,
    max_notes: values.max_notes,
    max_input_chars: values.max_input_chars,
    synthesize_audio: values.synthesize_audio !== false,
    tts_command: values.tts_command,
    tts_args: values.tts_args,
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
  {
    id: "lecture",
    category: "Study",
    label: "Notes to lecture",
    endpoint: "/api/workflows/study/lecture",
    fields: lectureFields,
    buildPayload: lecturePayload,
  },
];
