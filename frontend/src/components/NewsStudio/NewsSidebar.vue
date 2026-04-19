<template>
  <aside class="sidebar">
    <div class="sidebar-header">
      <h1>News Hub</h1>
      <div class="subtitle">AI Current Affairs Processing</div>
    </div>

    <div class="sidebar-actions" style="margin-top: 16px;">
      <button class="btn" :class="activeTab === 'process' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left; margin-bottom: 8px;" @click="$emit('update:activeTab', 'process')">
        📰 God-Mode RAG
      </button>
      <button class="btn" :class="activeTab === 'dedupe' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left;" @click="$emit('update:activeTab', 'dedupe')">
        ✨ Excel Deduplicator
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
defineProps<{ activeTab: string }>()
defineEmits<{ (e: 'update:activeTab', tab: string): void }>()
</script>
