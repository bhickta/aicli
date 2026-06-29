<script setup lang="ts">
import { computed } from "vue";
import { RouterLink } from "vue-router";
import { primaryRoutes, type AppRouteGroup } from "../../router";

const groups: AppRouteGroup[] = ["Work", "System"];

const groupedRoutes = computed(() => {
  return groups.map((group) => ({
    label: group,
    routes: primaryRoutes.filter((route) => route.meta?.group === group),
  }));
});
</script>

<template>
  <aside class="app-sidebar">
    <div class="app-brand">
      <strong>AICLI</strong>
      <span>Local AI Control Center</span>
    </div>
    <nav aria-label="Primary navigation">
      <div id="primary-tabs" aria-label="Main views">
        <section v-for="group in groupedRoutes" :key="group.label" class="nav-group">
          <h2 class="nav-group-label">{{ group.label }}</h2>
          <RouterLink
            v-for="route in group.routes"
            :id="`tab-${String(route.name)}`"
            :key="String(route.name)"
            :to="{ name: route.name }"
            class="nav-link"
          >
            <span class="nav-label">{{ route.meta?.label }}</span>
            <span class="nav-description">{{ route.meta?.description }}</span>
          </RouterLink>
        </section>
      </div>
    </nav>
  </aside>
</template>
