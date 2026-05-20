<script setup lang="ts">
import type { SystemResources } from "../types";

defineProps<{
  resources: SystemResources | null;
}>();

function percent(value: number | undefined) {
  const n = Number(value || 0);
  return `${Math.max(0, Math.min(100, n)).toFixed(0)}%`;
}

function gb(bytes: number | undefined) {
  return `${((bytes || 0) / 1024 / 1024 / 1024).toFixed(1)} GB`;
}
</script>

<template>
  <div class="system-usage" aria-label="System usage">
    <div class="usage-item">
      <span>CPU</span>
      <strong>{{ percent(resources?.cpu.usage_percent) }}</strong>
      <small>{{ resources?.cpu.logical_cores || 0 }} threads</small>
      <div class="usage-bar"><div :style="{ width: percent(resources?.cpu.usage_percent) }" /></div>
    </div>
    <div class="usage-item">
      <span>RAM</span>
      <strong>{{ percent(resources?.ram.usage_percent) }}</strong>
      <small>{{ gb(resources?.ram.used_bytes) }} / {{ gb(resources?.ram.total_bytes) }}</small>
      <div class="usage-bar"><div :style="{ width: percent(resources?.ram.usage_percent) }" /></div>
    </div>
    <div class="usage-item">
      <span>GPU</span>
      <strong>{{ resources?.gpus?.[0] ? percent(resources.gpus[0].utilization_percent) : "n/a" }}</strong>
      <small v-if="resources?.gpus?.[0]">{{ resources.gpus[0].memory_used_mb }} / {{ resources.gpus[0].memory_total_mb }} MB</small>
      <small v-else>No NVIDIA GPU</small>
      <div class="usage-bar"><div :style="{ width: resources?.gpus?.[0] ? percent(resources.gpus[0].utilization_percent) : '0%' }" /></div>
    </div>
  </div>
</template>
