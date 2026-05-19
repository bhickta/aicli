export const DOCUMENT_WORKFLOWS = [
  {
    id: "ocr-run",
    category: "Documents",
    label: "OCR: ZIP images to Markdown",
    endpoint: "/api/workflows/ocr/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input ZIP file" },
      { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
      { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      render_workers: values.render_workers,
      workers: values.workers,
    }),
  },
  {
    id: "ocr-pdf",
    category: "Documents",
    label: "OCR: PDF to Markdown",
    endpoint: "/api/workflows/ocr/pdf",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input PDF file" },
      { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
      { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      render_workers: values.render_workers,
      workers: values.workers,
    }),
  },
  {
    id: "analyze",
    category: "Documents",
    label: "Analyze: PDF report",
    endpoint: "/api/workflows/analyze/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input PDF file" },
      { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
      { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      render_workers: values.render_workers,
      workers: values.workers,
    }),
  },
];
