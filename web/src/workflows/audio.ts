import type { WorkflowDefinition } from "../types";
import { providerModelField, providerModelPayload } from "./builders";

export const audioWorkflowDefinitions: WorkflowDefinition[] = [
  {
    id: "audio-transcribe",
    category: "Audio",
    label: "Transcribe",
    endpoint: "/api/workflows/audio/transcribe",
    fields: [
      { type: "path", id: "path", label: "Input audio file" },
      { type: "text", id: "model", label: "Whisper model", value: "large-v3", placeholder: "large-v3" },
    ],
    buildPayload: (values) => ({
      model: values.model,
      path: values.path,
    }),
  },
  {
    id: "audio-analyze",
    category: "Audio",
    label: "Analyze tracks",
    endpoint: "/api/workflows/audio/analyze",
    fields: [
      providerModelField,
      {
        type: "textarea",
        id: "transcript",
        label: "Transcript",
        rows: 10,
        placeholder: "Paste transcript(s), separated by blank line delimiter `---`",
      },
    ],
    buildPayload: (values) => ({
      ...providerModelPayload(values),
      track_text:
        typeof values.transcript === "string"
          ? values.transcript
              .split("\n---\n")
              .map((text) => text.trim())
              .filter(Boolean)
          : [],
      track_names: [],
    }),
  },
];
