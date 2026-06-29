export interface WorkflowField {
  type: "providerModel" | "stepProviderModel" | "text" | "datetime" | "textarea" | "select" | "number" | "checkbox" | "path" | "whatsappContact";
  id?: string;
  label?: string;
  rows?: number;
  placeholder?: string;
  value?: string;
  default?: string | number;
  min?: number;
  max?: number;
  checked?: boolean;
  picker?: "directory";
  options?: Array<{ value: string; label: string }>;
}

export interface WorkflowDefinition {
  id: string;
  category: string;
  label: string;
  endpoint: string;
  preferredProviderId?: string;
  preferredModel?: string;
  fields: WorkflowField[];
  buildPayload: (values: Record<string, unknown>) => Record<string, unknown>;
}

export interface LectureResult {
  kind: "lecture";
  id: string;
  title: string;
  script: string;
  script_path: string;
  script_url: string;
  audio_path?: string;
  audio_url?: string;
  source_notes: string[];
  skipped_notes: number;
  input_chars: number;
  tts_command_line?: string[];
}
