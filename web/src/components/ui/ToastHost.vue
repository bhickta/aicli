<script setup lang="ts">
import { useToasts } from "../../composables/useToasts";

const { toasts, dismissToast } = useToasts();
</script>

<template>
  <div class="toast-host" aria-live="polite" aria-label="Notifications">
    <article v-for="toast in toasts" :key="toast.id" class="toast" :class="toast.kind">
      <div>
        <strong>{{ toast.title }}</strong>
        <p v-if="toast.message">{{ toast.message }}</p>
      </div>
      <button type="button" aria-label="Dismiss notification" @click="dismissToast(toast.id)">Dismiss</button>
    </article>
  </div>
</template>

<style scoped>
.toast-host {
  bottom: 1rem;
  display: grid;
  gap: 0.5rem;
  max-width: min(28rem, calc(100vw - 2rem));
  position: fixed;
  right: 1rem;
  z-index: 80;
}

.toast {
  align-items: start;
  background: #111827;
  border: 1px solid #334155;
  border-left-width: 3px;
  border-radius: 6px;
  box-shadow: 0 14px 36px #00000055;
  display: grid;
  gap: 0.75rem;
  grid-template-columns: minmax(0, 1fr) auto;
  padding: 0.7rem;
}

.toast.success {
  border-left-color: #4ade80;
}

.toast.error {
  border-left-color: #f87171;
}

.toast.info {
  border-left-color: #60a5fa;
}

.toast p {
  color: #cbd5e1;
  margin: 0.18rem 0 0;
}

.toast button {
  font-size: 0.78rem;
  padding: 0.25rem 0.45rem;
  text-align: center;
}
</style>
