export type ViewName = "chat" | "workflows" | "jobs" | "providers" | "tools" | "settings";

export interface ProviderConfig {
  id: string;
  type: string;
  name: string;
  base_url: string;
  api_key: string;
  model: string;
  headers?: Record<string, string>;
}

export interface Settings {
  default_provider: string;
  default_model: string;
  providers: ProviderConfig[];
  tools: Record<string, string>;
}

export interface Model {
  id: string;
  name: string;
}

export interface Job {
  id: string;
  type: string;
  status: string;
  stage: string;
  progress: number;
  current_step: number;
  total_steps: number;
  eta_seconds: number;
  input: string;
  output: string;
  error: string;
  created_at: string;
  updated_at: string;
  finished_at?: string;
}

export interface WorkflowField {
  type: "providerModel" | "text" | "textarea" | "select" | "number" | "checkbox" | "path";
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
  fields: WorkflowField[];
  buildPayload: (values: Record<string, unknown>) => Record<string, unknown>;
}

export interface BrowserEntry {
  name: string;
  path: string;
  is_dir: boolean;
}

export interface UploadedFile {
  name: string;
  path: string;
  url: string;
}
