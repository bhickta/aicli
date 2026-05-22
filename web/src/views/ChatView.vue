<script setup lang="ts">
import { shallowReactive, shallowRef, watch } from "vue";
import ProviderModelControl from "../components/ProviderModelControl.vue";
import { api } from "../lib/api";
import { readStoredString, writeStoredString } from "../lib/persistence";
import type { TokenUsage } from "../types";

const providerModel = shallowReactive({
  provider_id: readStoredString("aicli.chat.providerId"),
  model: readStoredString("aicli.chat.model"),
});
const prompt = shallowRef("");
const status = shallowRef("Ready");
const answer = shallowRef("");
const usage = shallowRef<TokenUsage | null>(null);
const busy = shallowRef(false);

watch(providerModel, () => {
  writeStoredString("aicli.chat.providerId", providerModel.provider_id);
  writeStoredString("aicli.chat.model", providerModel.model);
});

async function sendChat() {
  busy.value = true;
  status.value = "Thinking...";
  answer.value = "";
  usage.value = null;
  try {
    const result = await api<{ content: string; usage?: TokenUsage }>("/api/chat", {
      method: "POST",
      body: JSON.stringify({
        provider_id: providerModel.provider_id,
        model: providerModel.model,
        messages: [{ role: "user", content: prompt.value }],
        temperature: 0.1,
      }),
    });
    answer.value = result.content;
    usage.value = result.usage || null;
    status.value = "Done";
  } catch (error) {
    answer.value = error instanceof Error ? error.message : "Chat failed";
    status.value = "Failed";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="panel grid">
    <h2>Chat</h2>
    <ProviderModelControl
      :provider-id="providerModel.provider_id"
      :model="providerModel.model"
      @change="Object.assign(providerModel, $event)"
    />
    <div class="field">
      <label for="chat-prompt">Prompt</label>
      <textarea id="chat-prompt" v-model="prompt" rows="8" placeholder="Ask a local model..." />
    </div>
    <div class="field">
      <button id="chat-send" type="button" :disabled="busy" @click="sendChat">Send</button>
    </div>
    <div class="field">
      <h3>Status</h3>
      <p id="chat-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div class="field">
      <h3>Answer</h3>
      <p v-if="usage" class="status-line">
        Tokens: {{ usage.total_tokens || 0 }} total, {{ usage.input_tokens || 0 }} input,
        {{ usage.cached_input_tokens || 0 }} cached, {{ usage.output_tokens || 0 }} output
      </p>
      <pre id="chat-answer" role="status" aria-live="polite">{{ answer }}</pre>
    </div>
  </div>
</template>
