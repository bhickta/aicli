<script setup lang="ts">
import type { StudyQuestionFilterOptions, StudyQuestionFilters } from "../../lib/studyMetadata";

defineProps<{
  filters: StudyQuestionFilters;
  options: StudyQuestionFilterOptions;
  total: number;
  visible: number;
}>();

const emit = defineEmits<{
  update: [filters: Partial<StudyQuestionFilters>];
  clear: [];
}>();

function update(key: keyof StudyQuestionFilters, value: string) {
  emit("update", { [key]: value });
}
</script>

<template>
  <div class="study-question-filters">
    <input
      :value="filters.query"
      type="search"
      placeholder="Search questions, topics, answer text"
      @input="update('query', ($event.target as HTMLInputElement).value)"
    />
    <select :value="filters.subject" @change="update('subject', ($event.target as HTMLSelectElement).value)">
      <option value="">All subjects</option>
      <option v-for="subject in options.subjects" :key="subject" :value="subject">{{ subject }}</option>
    </select>
    <select :value="filters.topic" @change="update('topic', ($event.target as HTMLSelectElement).value)">
      <option value="">All topics</option>
      <option v-for="topic in options.topics" :key="topic" :value="topic">{{ topic }}</option>
    </select>
    <select :value="filters.paper" @change="update('paper', ($event.target as HTMLSelectElement).value)">
      <option value="">All papers</option>
      <option v-for="paper in options.papers" :key="paper" :value="paper">{{ paper }}</option>
    </select>
    <select :value="filters.difficulty" @change="update('difficulty', ($event.target as HTMLSelectElement).value)">
      <option value="">All difficulty</option>
      <option v-for="difficulty in options.difficulties" :key="difficulty" :value="difficulty">{{ difficulty }}</option>
    </select>
    <button type="button" class="study-btn-action secondary" @click="emit('clear')">Clear</button>
    <span class="study-filter-count">{{ visible }} / {{ total }}</span>
  </div>
</template>
