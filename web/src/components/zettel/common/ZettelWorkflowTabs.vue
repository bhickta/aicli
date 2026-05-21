<script setup lang="ts">
import type { ZettelMode } from "../../../features/zettel/types";

const mode = defineModel<ZettelMode>({ required: true });

const tabs: Array<{ id: ZettelMode; label: string }> = [
  { id: "inbox", label: "Inbox merge" },
  { id: "manual", label: "Manual review" },
  { id: "settings", label: "Settings" },
];
</script>

<template>
  <div class="zettel-workflow-tabs" role="tablist" aria-label="Zettelkasten workflow">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      role="tab"
      :aria-selected="mode === tab.id"
      :class="{ active: mode === tab.id }"
      @click="mode = tab.id"
    >
      {{ tab.label }}
    </button>
  </div>
</template>

<style scoped>
.zettel-workflow-tabs {
  display: inline-flex;
  gap: 4px;
  padding: 4px;
  border: 1px solid #2b313b;
  border-radius: 8px;
  background: #10141b;
}

.zettel-workflow-tabs button {
  border-color: transparent;
  background: transparent;
  color: #9aa4b2;
  text-align: center;
  white-space: nowrap;
}

.zettel-workflow-tabs button.active {
  border-color: #6ea8fe;
  background: #2d405e;
  color: #ffffff;
}

@media (max-width: 640px) {
  .zettel-workflow-tabs {
    display: flex;
    width: 100%;
    overflow-x: auto;
  }

  .zettel-workflow-tabs button {
    flex: 1 0 auto;
  }
}
</style>
