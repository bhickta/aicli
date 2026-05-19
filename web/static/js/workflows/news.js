export const NEWS_WORKFLOWS = [
  {
    id: "news",
    category: "News",
    label: "Analyze news feed",
    endpoint: "/api/workflows/news/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input JSON/XLSX file" },
      { type: "path", id: "output_path", label: "Output XLSX (optional)" },
      { type: "checkbox", id: "use_llm", label: "Use LLM summary", checked: true },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      output_path: values.output_path,
      use_llm: Boolean(values.model) && Boolean(values.use_llm),
    }),
  },
];
