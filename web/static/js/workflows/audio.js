export const AUDIO_WORKFLOWS = [
  {
    id: "audio-transcribe",
    category: "Audio",
    label: "Transcribe",
    endpoint: "/api/workflows/audio/transcribe",
    fields: [
      { type: "path", id: "path", label: "Input audio file" },
      { type: "text", id: "model", label: "Whisper model path (optional)", value: "", placeholder: "Leave blank to use whisper-cli default" },
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
      { type: "providerModel" },
      { type: "textarea", id: "transcript", label: "Transcript", rows: 10, placeholder: "Paste transcript(s), separated by blank line delimiter `---`" },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      track_text: values.transcript ? values.transcript.split("\n---\n").map((text) => text.trim()).filter(Boolean) : [],
      track_names: [],
    }),
  },
];
