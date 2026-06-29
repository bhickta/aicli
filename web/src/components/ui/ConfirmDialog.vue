<script setup lang="ts">
import { computed } from "vue";
import { useConfirm } from "../../composables/useConfirm";

const { activeConfirm, resolveConfirm } = useConfirm();

const confirmClass = computed(() => ({
  danger: Boolean(activeConfirm.value?.danger),
}));
</script>

<template>
  <Teleport to="body">
    <div v-if="activeConfirm" class="confirm-backdrop" role="presentation" @click.self="resolveConfirm(false)">
      <section class="confirm-dialog" role="dialog" aria-modal="true" aria-labelledby="confirm-title">
        <h2 id="confirm-title">{{ activeConfirm.title }}</h2>
        <p>{{ activeConfirm.message }}</p>
        <div class="confirm-actions">
          <button type="button" @click="resolveConfirm(false)">
            {{ activeConfirm.cancelLabel || "Cancel" }}
          </button>
          <button type="button" :class="confirmClass" @click="resolveConfirm(true)">
            {{ activeConfirm.confirmLabel || "Confirm" }}
          </button>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.confirm-backdrop {
  align-items: center;
  background: #020617aa;
  display: grid;
  inset: 0;
  justify-items: center;
  padding: 1rem;
  position: fixed;
  z-index: 90;
}

.confirm-dialog {
  background: #111827;
  border: 1px solid #334155;
  border-radius: 7px;
  box-shadow: 0 22px 70px #00000080;
  display: grid;
  gap: 0.8rem;
  max-width: min(28rem, 100%);
  padding: 1rem;
}

.confirm-dialog h2,
.confirm-dialog p {
  margin: 0;
}

.confirm-dialog p {
  color: #cbd5e1;
}

.confirm-actions {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
}

.confirm-actions button {
  text-align: center;
}

.confirm-actions button.danger {
  background: #3b1115;
  border-color: #7f1d1d;
  color: #fecaca;
}
</style>
