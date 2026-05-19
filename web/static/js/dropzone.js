import { collectDroppedFiles } from "./dropped-files.js";
import { uploadWorkflowFiles } from "./workflow-upload.js";

export function renderDropZone() {
  return `
    <div id="drop-zone" class="drop-zone" tabindex="0">
      <strong>Drop a file to auto-select workflow</strong>
      <span>Single files upload to app storage. For in-place video course folders, use Choose Course source folder.</span>
    </div>
  `;
}

export function setupDropZone(renderPage) {
  const dropZone = document.querySelector("#drop-zone");
  if (!dropZone) return;
  const stop = (event) => {
    event.preventDefault();
    event.stopPropagation();
  };
  ["dragenter", "dragover"].forEach((eventName) => {
    dropZone.addEventListener(eventName, (event) => {
      stop(event);
      dropZone.classList.add("dragover");
    });
  });
  ["dragleave", "drop"].forEach((eventName) => {
    dropZone.addEventListener(eventName, (event) => {
      stop(event);
      dropZone.classList.remove("dragover");
    });
  });
  dropZone.addEventListener("drop", async (event) => {
    const entries = await collectDroppedFiles(event.dataTransfer);
    if (!entries.length) return;
    await uploadWorkflowFiles(entries, renderPage);
  });
}
