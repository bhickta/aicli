export function setSourcePreview(url) {
  const pane = document.querySelector("#review-pane");
  const frame = document.querySelector("#source-preview");
  if (!pane || !frame) return;
  if (!url) {
    frame.removeAttribute("src");
    return;
  }
  frame.src = url;
  pane.classList.remove("hidden");
}

export function setMarkdownPreview(markdown) {
  const pane = document.querySelector("#review-pane");
  const preview = document.querySelector("#markdown-preview");
  if (!pane || !preview) return;
  preview.value = markdown || "";
  if (markdown || document.querySelector("#source-preview")?.getAttribute("src")) {
    pane.classList.remove("hidden");
  }
}
