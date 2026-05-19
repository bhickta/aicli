import { api } from "../core/api.js";
import { view } from "../core/elements.js";
import { getProviderModelValues, renderProviderModelRow, setupProviderModel } from "../providers/controls.js";

export function renderChat() {
  view.innerHTML = `
    <div class="panel grid">
      <h2>Chat</h2>
      ${renderProviderModelRow("chat")}
      <div class="field">
        <label for="chat-prompt">Prompt</label>
        <textarea id="chat-prompt" rows="8" placeholder="Ask a local model..."></textarea>
      </div>
      <div class="field">
        <button id="chat-send" type="button">Send</button>
      </div>
      <div class="field">
        <h3>Status</h3>
        <p id="chat-status" class="status-line" role="status" aria-live="polite">Ready</p>
      </div>
      <div class="field">
        <h3>Answer</h3>
        <pre id="chat-answer" role="status" aria-live="polite"></pre>
      </div>
    </div>
  `;
  setupProviderModel("chat");
  document.querySelector("#chat-send").addEventListener("click", sendChat);
}

async function sendChat() {
  const prompt = document.querySelector("#chat-prompt");
  const status = document.querySelector("#chat-status");
  const answer = document.querySelector("#chat-answer");
  const { provider_id, model } = getProviderModelValues("chat");
  status.textContent = "Thinking...";
  answer.textContent = "";
  try {
    const result = await api("/api/chat", {
      method: "POST",
      body: JSON.stringify({
        provider_id,
        model,
        messages: [{ role: "user", content: prompt.value }],
        temperature: 0.1,
      }),
    });
    status.textContent = "Done";
    answer.textContent = result.content;
  } catch (error) {
    status.textContent = "Failed";
    answer.textContent = error.message;
  }
}
