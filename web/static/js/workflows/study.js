export const STUDY_WORKFLOWS = [
  {
    id: "recall",
    category: "Study",
    label: "Recall",
    endpoint: "/api/workflows/recall/run",
    fields: [
      { type: "providerModel" },
      { type: "textarea", id: "notes", label: "Notes", rows: 12, placeholder: "Paste UPSC notes..." },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      notes: values.notes,
    }),
  },
];
