<template>
  <aside class="sidebar">
    <div class="sidebar-header">
      <h1>Image Studio</h1>
      <div class="subtitle">AI Vision Processing</div>
    </div>

    <div class="sidebar-actions" style="margin-top: 16px;">
      <button class="btn" :class="activeTab === 'rename' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left; margin-bottom: 8px;" @click="$emit('update:activeTab', 'rename')">
        🪄 AI Renamer
      </button>
      <button class="btn" :class="activeTab === 'clean' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left; margin-bottom: 8px;" @click="$emit('update:activeTab', 'clean')">
        🧹 Junk Cleaner
      </button>
      <button class="btn" :class="activeTab === 'digitize' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left;" @click="$emit('update:activeTab', 'digitize')">
        📝 Markdown Digitize
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
defineProps<{ activeTab: string }>()
defineEmits<{ (e: 'update:activeTab', tab: string): void }>()
</script>
