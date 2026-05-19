export const IMAGE_WORKFLOWS = [
  {
    id: "image-run",
    category: "Images",
    label: "Analyze image",
    endpoint: "/api/workflows/image/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input image" },
      {
        type: "select",
        id: "mode",
        label: "Mode",
        default: "",
        options: [
          { value: "", label: "Default (safe rename)" },
          { value: "rename", label: "Rename" },
          { value: "junk", label: "Junk classification" },
          { value: "digitize", label: "Digitize text" },
        ],
      },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      mode: values.mode || "",
    }),
  },
  {
    id: "image-rename",
    category: "Images",
    label: "Safe image rename",
    endpoint: "/api/workflows/image/rename",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input image" },
      { type: "checkbox", id: "apply", label: "Apply rename", checked: false },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      apply: Boolean(values.apply),
    }),
  },
  {
    id: "image-prune-refs",
    category: "Images",
    label: "Prune stale refs",
    endpoint: "/api/workflows/image/prune-refs",
    fields: [
      { type: "path", id: "markdown_path", label: "Markdown file" },
      { type: "path", id: "asset_dir", label: "Asset directory" },
      { type: "checkbox", id: "apply", label: "Apply filesystem changes", checked: false },
    ],
    buildPayload: (values) => ({
      markdown_path: values.markdown_path,
      asset_dir: values.asset_dir,
      apply: Boolean(values.apply),
    }),
  },
];
