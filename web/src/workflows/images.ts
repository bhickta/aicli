import type { WorkflowDefinition } from "../types";
import { pathPayload, providerModelField, providerPathPayload } from "./builders";

export const imageWorkflowDefinitions: WorkflowDefinition[] = [
  {
    id: "image-run",
    category: "Images",
    label: "Analyze image",
    endpoint: "/api/workflows/image/run",
    fields: [
      providerModelField,
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
      ...providerPathPayload(values),
      mode: values.mode || "",
    }),
  },
  {
    id: "image-rename",
    category: "Images",
    label: "Safe image rename",
    endpoint: "/api/workflows/image/rename",
    fields: [
      providerModelField,
      { type: "path", id: "path", label: "Input image" },
      { type: "checkbox", id: "apply", label: "Apply rename", checked: false },
    ],
    buildPayload: (values) => ({
      ...providerPathPayload(values),
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
      ...pathPayload(values, "markdown_path"),
      asset_dir: values.asset_dir,
      apply: Boolean(values.apply),
    }),
  },
];
