import { state } from "../../core/state.js";
import { setSourcePreview } from "../../views/previews.js";
import { setWorkflowPathValue } from "../form.js";
import { getActiveWorkflow } from "../utils.js";

export async function uploadWorkflowFile(file, renderPage) {
  return uploadWorkflowFiles([{ file, relativePath: file.name }], renderPage);
}

export async function uploadWorkflowFiles(entries, renderPage) {
  const status = document.querySelector("#workflow-status");
  const output = document.querySelector("#workflow-result");
  const dropZone = document.querySelector("#drop-zone");
  if (entries.some((entry) => (entry.relativePath || "").includes("/"))) {
    await switchToInPlaceCourseFlow(renderPage);
    return;
  }

  const uploadLabel = entries.length === 1 ? entries[0].file.name : `${entries.length} files`;
  if (status) status.textContent = `Uploading ${uploadLabel}...`;
  if (output) output.textContent = "";
  if (dropZone) dropZone.classList.add("uploading");
  try {
    const result = await uploadEntries(entries);
    const uploadedFiles = result.files || [];
    const uploaded = uploadedFiles[0];
    if (!uploaded) throw new Error("upload finished without a stored file");
    if (hasUploadedCourseFolder(uploadedFiles, result.root)) {
      await selectUploadedCourseFolder(uploadedFiles, result.root, renderPage);
      return;
    }
    await autoSelectWorkflow(uploaded.name || entries[0].file.name, renderPage);
    selectPrimaryWorkflowPath(uploaded, entries[0].file.name);
  } catch (error) {
    const currentStatus = document.querySelector("#workflow-status");
    const currentOutput = document.querySelector("#workflow-result");
    if (currentStatus) currentStatus.textContent = "Upload failed";
    if (currentOutput) currentOutput.textContent = error.message;
  } finally {
    const currentDropZone = document.querySelector("#drop-zone") || dropZone;
    if (currentDropZone) currentDropZone.classList.remove("uploading");
  }
}

async function switchToInPlaceCourseFlow(renderPage) {
  state.workflow.category = "Video";
  state.workflow.workflowId = "video-course";
  if (state.view === "workflows") {
    await renderPage();
  }
  const status = document.querySelector("#workflow-status");
  const output = document.querySelector("#workflow-result");
  if (status) status.textContent = "Folder not uploaded";
  if (output) {
    output.textContent = "For in-place processing, click Choose Course source folder, navigate to the folder, then use current directory.";
  }
}

async function uploadEntries(entries) {
  const form = new FormData();
  entries.forEach((entry) => {
    const relativePath = entry.relativePath || entry.file.name;
    const fieldName = relativePath.includes("/") ? `file:${relativePath}` : "file";
    form.append(fieldName, entry.file, entry.file.name);
  });
  const response = await fetch("/api/fs/upload", { method: "POST", body: form });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json();
}

function hasUploadedCourseFolder(uploadedFiles, root) {
  return Boolean(root) && uploadedFiles.length > 1 && uploadedFiles.some((file) => isVideoFileName(file.name));
}

async function selectUploadedCourseFolder(uploadedFiles, root, renderPage) {
  state.workflow.category = "Video";
  state.workflow.workflowId = "video-course";
  if (state.view === "workflows") {
    await renderPage();
  }
  setWorkflowPathValue("path", root);
  const status = document.querySelector("#workflow-status");
  if (status) status.textContent = `Ready: uploaded folder with ${uploadedFiles.length} files`;
}

function selectPrimaryWorkflowPath(uploaded, fallbackName) {
  const workflow = getActiveWorkflow();
  const primaryPathField = workflow.fields.find((field) => field.type === "path");
  if (primaryPathField) {
    setWorkflowPathValue(primaryPathField.id, uploaded.path);
    setSourcePreview(uploaded.url);
  }
  const status = document.querySelector("#workflow-status");
  if (status) {
    status.textContent = `Ready: ${uploaded.name || fallbackName}`;
  }
}

async function autoSelectWorkflow(fileName, renderPage) {
  const lower = fileName.toLowerCase();
  if (lower.endsWith(".pdf")) {
    state.workflow.workflowId = "ocr-pdf";
    state.workflow.category = "Documents";
  } else if (lower.endsWith(".zip")) {
    state.workflow.workflowId = "ocr-run";
    state.workflow.category = "Documents";
  } else if (/\.(png|jpe?g|webp|gif|bmp|tiff?)$/.test(lower)) {
    state.workflow.workflowId = "image-run";
    state.workflow.category = "Images";
  } else if (/\.(mp3|wav|m4a|flac|ogg|opus)$/.test(lower)) {
    state.workflow.workflowId = "audio-transcribe";
    state.workflow.category = "Audio";
  } else if (/\.(mp4|mov|mkv|webm|avi|m4v)$/.test(lower)) {
    state.workflow.workflowId = "video-course";
    state.workflow.category = "Video";
  }
  if (state.view === "workflows") {
    await renderPage();
  }
}

function isVideoFileName(name) {
  return /\.(mp4|mov|mkv|webm|avi|m4v)$/i.test(name || "");
}
