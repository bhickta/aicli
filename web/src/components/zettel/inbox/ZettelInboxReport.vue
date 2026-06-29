<script setup lang="ts">
import "../../../styles/zettel-inbox-report.css";
import { computed, shallowRef } from "vue";
import type { InboxMergeReport } from "../../../types";
import ZettelBadges from "../common/ZettelBadges.vue";
import ZettelDestinationDiff from "./ZettelDestinationDiff.vue";

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
const selectedLedger = computed(() => selected.value?.ledger || []);
const selectedClaims = computed(() => selected.value?.claims || []);
const reportBadges = computed(() => {
  if (!props.report) return [];
  return [
    `${props.report.selected_count ?? props.report.processed_count + props.report.pending_count + props.report.failed_count} selected`,
    `${props.report.source_count ?? 0} in inbox`,
    `${props.report.skipped_count ?? 0} skipped`,
    `${props.report.processed_count} processed`,
    `${props.report.pending_count} pending`,
    `${props.report.failed_count} failed`,
  ];
});
const apiBadges = computed(() => {
  if (!props.report?.api_calls) return [];
  return [
    `${props.report.api_calls.chat} chat`,
    `${props.report.api_calls.embeddings} embeddings`,
    `${props.report.api_calls.total} calls`,
  ];
});

function selectRow(path: string) {
  selectedSource.value = path;
}

function fileName(path: string): string {
  return path.split("/").pop() || path;
}

function parentPath(path: string): string {
  const parts = path.split("/");
  parts.pop();
  return parts.join("/");
}
</script>

<template>
  <section v-if="report" class="inbox-report">
    <header class="inbox-report-header">
      <div>
        <h3>Inbox Merge Audit</h3>
        <p class="muted">{{ report.run_id }}</p>
      </div>
      <div class="inbox-report-badges">
        <ZettelBadges :items="reportBadges" />
        <ZettelBadges v-if="apiBadges.length" :items="apiBadges" />
      </div>
    </header>

    <div class="inbox-workbench">
      <aside class="inbox-source-list" aria-label="Inbox source notes">
        <div class="source-list-header">
          <strong>Source notes</strong>
          <span>{{ rows.length }}</span>
        </div>
        <button
          v-for="row in rows"
          :key="row.source_path"
          type="button"
          class="source-row"
          :class="{ active: selected?.source_path === row.source_path }"
          @click="selectRow(row.source_path)"
        >
          <span class="source-state" :class="`is-${row.status}`">{{ row.status }}</span>
          <span class="source-copy">
            <strong>{{ fileName(row.source_path) }}</strong>
            <small>{{ parentPath(row.source_path) }}</small>
            <em>
              {{ row.merged_count }} merged
              <template v-if="row.deduped_count"> | {{ row.deduped_count }} deduped</template>
              <template v-if="row.pending_count"> | {{ row.pending_count }} pending</template>
            </em>
          </span>
        </button>
      </aside>

      <article v-if="selected" class="inbox-detail">
        <header class="source-summary">
          <div>
            <strong>{{ selected.source_path }}</strong>
            <p class="status-line compact">
              {{ selected.status }}
              <template v-if="selected.processed_path"> -> {{ selected.processed_path }}</template>
            </p>
          </div>
          <ZettelBadges :items="[`${selected.diffs?.length || 0} changed`, `${selected.merged_count} merged`, `${selected.pending_count} pending`]" />
        </header>
        <p v-if="selected.reason" class="source-reason">{{ selected.reason }}</p>

        <ZettelDestinationDiff
          :diffs="selected.diffs || []"
          :source-path="selected.source_path"
          :source-content="selected.source_content"
          :processed-path="selected.processed_path"
        />

        <details v-if="selectedClaims.length || selectedLedger.length" class="ledger-details">
          <summary>Merge ledger</summary>
          <div class="claim-ledger">
            <article v-for="claim in selectedClaims" :key="claim.id" class="claim-row">
              <strong>{{ claim.id }}</strong>
              <p>{{ claim.text }}</p>
              <small v-if="claim.source">{{ claim.source }}</small>
              <ul>
                <li v-for="item in selectedLedger.filter((entry) => entry.claim_id === claim.id)" :key="`${item.claim_id}-${item.status}-${item.destination_path || item.reason || ''}`">
                  {{ item.status }}
                  <template v-if="item.destination_path"> -> {{ item.destination_path }}</template>
                  <template v-if="item.evidence"> | {{ item.evidence }}</template>
                  <template v-if="item.reason"> | {{ item.reason }}</template>
                </li>
              </ul>
            </article>
            <article v-if="!selectedClaims.length" class="claim-row">
              <strong>Ledger</strong>
              <ul>
                <li v-for="item in selectedLedger" :key="`${item.claim_id}-${item.status}-${item.destination_path || item.reason || ''}`">
                  {{ item.claim_id }} | {{ item.status }}
                  <template v-if="item.destination_path"> -> {{ item.destination_path }}</template>
                  <template v-if="item.evidence"> | {{ item.evidence }}</template>
                  <template v-if="item.reason"> | {{ item.reason }}</template>
                </li>
              </ul>
            </article>
          </div>
        </details>
      </article>
    </div>
  </section>
</template>
