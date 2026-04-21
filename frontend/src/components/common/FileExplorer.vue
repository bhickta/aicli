<template>
  <div v-if="show" class="explorer-overlay" @click.self="$emit('close')">
    <div class="explorer-modal">
      <div class="explorer-header">
        <h3>Select Directory</h3>
        <button class="btn-close" @click="$emit('close')">✕</button>
      </div>

      <div class="explorer-nav">
        <button class="btn-nav" @click="goUp" :disabled="!parentDir">⬆ Up</button>
        <span class="current-path">{{ currentDir }}</span>
      </div>

      <div class="explorer-list">
        <div 
          v-for="item in items" 
          :key="item.path" 
          class="explorer-item"
          :class="{ 'is-dir': item.is_dir }"
          @click="handleClick(item)"
        >
          <span class="item-icon">{{ item.is_dir ? '📁' : '📄' }}</span>
          <span class="item-name">{{ item.name }}</span>
          <span class="item-size" v-if="!item.is_dir">{{ formatSize(item.size) }}</span>
        </div>
      </div>

      <div class="explorer-footer">
        <button class="btn btn-primary" @click="confirm">Select This Folder</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { fsApi } from '../../api/FSApiClient'

const props = defineProps<{ show: boolean, initialPath?: string }>()
const emit = defineEmits<{ (e: 'close'): void, (e: 'select', path: string): void }>()

const currentDir = ref('')
const parentDir = ref('')
const items = ref<any[]>([])

async function loadDir(path: string) {
  try {
    const data = await fsApi.listDir(path)
    items.value = data.items
    currentDir.value = data.current
    parentDir.value = data.parent
  } catch (e) {
    console.error('Failed to load directory:', e)
  }
}

function handleClick(item: any) {
  if (item.is_dir) {
    loadDir(item.path)
  }
}

function goUp() {
  if (parentDir.value) {
    loadDir(parentDir.value)
  }
}

function confirm() {
  emit('select', currentDir.value)
  emit('close')
}

function formatSize(bytes: number) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

watch(() => props.show, (newShow) => {
  if (newShow) {
    loadDir(props.initialPath || '/')
  }
})

onMounted(() => {
  if (props.show) loadDir(props.initialPath || '/')
})
</script>

<style scoped>
.explorer-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(4px);
}

.explorer-modal {
  background: #1e1e2e;
  width: 600px;
  height: 500px;
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.explorer-header {
  padding: 16px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.explorer-nav {
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  display: flex;
  gap: 12px;
  align-items: center;
}

.current-path {
  font-family: monospace;
  font-size: 13px;
  opacity: 0.7;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.explorer-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.explorer-item {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  border-radius: 6px;
  cursor: pointer;
  gap: 12px;
  transition: background 0.2s;
}

.explorer-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.item-icon { font-size: 18px; }
.item-name { flex: 1; font-size: 14px; }
.item-size { font-size: 12px; opacity: 0.5; }

.explorer-footer {
  padding: 16px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  justify-content: flex-end;
}

.btn-close {
  background: none;
  border: none;
  color: white;
  cursor: pointer;
  font-size: 18px;
  opacity: 0.5;
}

.btn-nav {
  background: rgba(255, 255, 255, 0.1);
  border: none;
  color: white;
  padding: 4px 12px;
  border-radius: 4px;
  cursor: pointer;
}

.btn-nav:disabled { opacity: 0.3; cursor: not-allowed; }
</style>
