<script setup lang="ts">
import { computed, shallowRef } from "vue";
import type { InboxMergeReport } from "../../types";
import ZettelBadges from "./ZettelBadges.vue";

const props = defineProps<{
  report: InboxMergeReport | null;
}>();

const selectedSource = shallowRef("");
const rows = computed(() => {
  if (!props.report) return [];
  return [
    ...(props.report.processed || []),
    ...(props.report.pending || []),
    ...(props.report.failed || []),
  ];
});
const selected = computed(() => rows.value.find((row) => row.source_path === selectedSource.value) || rows.value[0] || null);
const reportBadges = computed(() => {
  if (!props.report) return [];
  return [
    `${props.report.processed_count} processed`,
    `${props.report.pending_count} pending`,
    `${props.report.failed_count} failed`,
  ];
});

function selectRow(path: string) {
  selectedSource.value = path;
}
</script>

<template>
  <section v-if="report" class="inbox-report">
    <header class="inbox-report-header">
      <div>
        <h3>Inbox Merge Audit</h3>
        <p class="muted">{{ report.run_id }}</p>
      </div>
      <ZettelBadges :items="reportBadges" />
    </header>

    <div class="inbox-report-grid">
      <div class="inbox-report-list">
        <button
          v-for="row in rows"
          :key="row.source_path"
          type="button"
          class="inbox-report-row"
          :class="{ active: selected?.source_path === row.source_path }"
          @click="selectRow(row.source_path)"
        >
          <strong>{{ row.source_path }}</strong>
          <span>{{ row.status }} | +{{ row.merged_count }} merged | {{ row.deduped_count }} deduped | {{ row.pending_count }} pending</span>
          <small>{{ row.destination_paths?.join(", ") || row.reason || "No destination" }}</small>
        </button>
      </div>

      <article v-if="selected" class="inbox-report-detail">
        <h4>{{ selected.source_path }}</h4>
        <p class="status-line compact">
          {{ selected.status }}
          <template v-if="selected.processed_path"> -> {{ selected.processed_path }}</template>
        </p>
        <p v-if="selected.reason" class="muted">{{ selected.reason }}</p>

        <details open>
          <summary>Claim ledger</summary>
          <div class="claim-ledger">
            <article v-for="claim in selected.claims || []" :key="claim.id" class="claim-row">
              <strong>{{ claim.id }}</strong>
              <p>{{ claim.text }}</p>
              <small v-if="claim.source">{{ claim.source }}</small>
              <ul>
                <li v-for="item in (selected.ledger || []).filter((entry) => entry.claim_id === claim.id)" :key="`${item.claim_id}-${item.status}-${item.destination_path || item.reason || ''}`">
                  {{ item.status }}
                  <template v-if="item.destination_path"> -> {{ item.destination_path }}</template>
                  <template v-if="item.evidence"> | {{ item.evidence }}</template>
                  <template v-if="item.reason"> | {{ item.reason }}</template>
                </li>
              </ul>
            </article>
          </div>
        </details>

        <details v-for="diff in selected.diffs || []" :key="diff.path">
          <summary>{{ diff.path }}</summary>
          <pre>{{ diff.diff || "No textual diff; claim may have been deduped." }}</pre>
        </details>
      </article>
    </div>
  </section>
</template>

<style scoped>
.inbox-report {
  display: grid;
  gap: 12px;
  padding: 0;
}

.inbox-report-header {
  margin: 0;
}

.inbox-report-grid {
  display: grid;
  grid-template-columns: minmax(240px, 0.8fr) minmax(0, 1.2fr);
  gap: 12px;
  min-width: 0;
}

.inbox-report-list,
.inbox-report-detail,
.claim-ledger {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.inbox-report-row {
  display: grid;
  gap: 4px;
  text-align: left;
}

.inbox-report-row.active {
  border-color: #6ea8fe;
  background: #2d405e;
}

.inbox-report-row span,
.inbox-report-row small,
.claim-row small {
  color: #9aa4b2;
}

.inbox-report-detail {
  padding: 12px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #10141b;
}

.claim-row {
  display: grid;
  gap: 4px;
  padding: 10px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #161b22;
}

.claim-row p,
.claim-row ul {
  margin: 0;
}

.claim-row ul {
  padding-left: 18px;
}

@media (max-width: 820px) {
  .inbox-report-grid {
    grid-template-columns: 1fr;
  }
}
</style>
