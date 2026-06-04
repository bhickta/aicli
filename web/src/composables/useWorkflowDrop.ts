import type { ComputedRef, Ref } from "vue";
import { selectWorkflowCategory } from "../stores/appState";
import type { UploadedFile, WorkflowDefinition } from "../types";
import type { DropEntry } from "../workflows/drop";

interface WorkflowDropOptions {
  activeWorkflow: ComputedRef<WorkflowDefinition | undefined>;
  updateField: (id: string, value: unknown) => void;
  chooseWorkflow: (id: string) => void;
  status: Ref<string>;
  result: Ref<string>;
  sourcePreview: Ref<string>;
  autoSelectWorkflow?: (fileName: string) => void;
}

export function useWorkflowDrop(options: WorkflowDropOptions) {
  async function handleDrop(entries: DropEntry[]) {
    if (!entries.length) {
      options.status.value = "No files selected";
      options.result.value = "";
      return;
    }
    if (entries.some((entry) => entry.relativePath.includes("/"))) {
      selectWorkflowCategory("Video");
      options.chooseWorkflow("video-course");
      options.status.value = "Folder not uploaded";
      options.result.value = "For in-place processing, click Choose Course source folder, navigate to the folder, then use current directory.";
      return;
    }
    const label = entries.length === 1 ? entries[0].file.name : `${entries.length} files`;
    options.status.value = `Uploading ${label}...`;
    options.result.value = "";
    try {
      const upload = await uploadEntries(entries);
      const uploaded = upload.files[0];
      if (!uploaded) throw new Error("upload finished without a stored file");
      const fileName = uploaded.name || entries[0].file.name;
      if (options.autoSelectWorkflow) {
        options.autoSelectWorkflow(fileName);
      } else {
        autoSelectWorkflow(fileName);
      }
      const primaryPathField = options.activeWorkflow.value?.fields.find((field) => field.type === "path");
      if (primaryPathField?.id) options.updateField(primaryPathField.id, uploaded.path);
      options.sourcePreview.value = uploaded.url || "";
      options.status.value = `Ready: ${uploaded.name || entries[0].file.name}`;
    } catch (error) {
      options.status.value = "Upload failed";
      options.result.value = error instanceof Error ? error.message : "Upload failed";
    }
  }

  function autoSelectWorkflow(fileName: string) {
    const lower = fileName.toLowerCase();
    if (lower.endsWith(".pdf")) {
      selectWorkflowCategory("Documents");
      options.chooseWorkflow("ocr-pdf");
    } else if (lower.endsWith(".zip")) {
      selectWorkflowCategory("Documents");
      options.chooseWorkflow("ocr-run");
    } else if (/\.(png|jpe?g|webp|gif|bmp|tiff?)$/.test(lower)) {
      selectWorkflowCategory("Images");
      options.chooseWorkflow("image-run");
    } else if (/\.(mp3|wav|m4a|flac|ogg|opus)$/.test(lower)) {
      selectWorkflowCategory("Audio");
      options.chooseWorkflow("audio-transcribe");
    } else if (/\.(mp4|mov|mkv|webm|avi|m4v)$/.test(lower)) {
      selectWorkflowCategory("Video");
      options.chooseWorkflow("video-course");
    }
  }

  return { handleDrop, autoSelectWorkflow };
}

async function uploadEntries(entries: DropEntry[]) {
  const form = new FormData();
  entries.forEach((entry) => {
    const fieldName = entry.relativePath.includes("/") ? `file:${entry.relativePath}` : "file";
    form.append(fieldName, entry.file, entry.file.name);
  });
  const response = await fetch("/api/fs/upload", { method: "POST", body: form });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json() as Promise<{ root: string; files: UploadedFile[] }>;
}
