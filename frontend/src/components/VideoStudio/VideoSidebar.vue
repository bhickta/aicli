<template>
  <aside class="sidebar">
    <div class="sidebar-header">
      <h1>Video Studio</h1>
      <div class="subtitle">Course Processing & Tagging</div>
    </div>

    <div class="sidebar-actions" style="margin-top: 16px;">
      <button class="btn" :class="activeTab === 'course' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left; margin-bottom: 8px;" @click="$emit('update:activeTab', 'course')">
        🎓 God-Mode Course
      </button>
      <button class="btn" :class="activeTab === 'compress' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left; margin-bottom: 8px;" @click="$emit('update:activeTab', 'compress')">
        ⚡ GPU Compress
      </button>
      <button class="btn" :class="activeTab === 'tag' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left; margin-bottom: 8px;" @click="$emit('update:activeTab', 'tag')">
        🏷️ AI Tagging
      </button>
      <button class="btn" :class="activeTab === 'notes' ? 'btn-primary' : 'btn-ghost'" style="width: 100%; text-align: left;" @click="$emit('update:activeTab', 'notes')">
        📝 Study Notes
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
defineProps<{ activeTab: string }>()
defineEmits<{ (e: 'update:activeTab', tab: string): void }>()
</script>
