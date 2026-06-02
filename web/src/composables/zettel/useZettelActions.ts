import { buildZettelMetadataPayload, buildZettelPayload, buildZettelTrainingPayload } from "../../features/zettel/payload";
import type { ZettelConfig } from "../../features/zettel/types";
import { stringify } from "../../lib/format";
import type {
  InboxCandidatePreviewReport,
  InboxMergeReport,
  MetadataReport,
  TrainingExportReport,
} from "../../types";
import type { useZettelRunner } from "../useZettelRunner";
import type { useZettelReports } from "./useZettelReports";

type ZettelRunner = ReturnType<typeof useZettelRunner>;
type ZettelReports = ReturnType<typeof useZettelReports>;

export function useZettelActions(config: ZettelConfig, runner: ZettelRunner, reports: ZettelReports) {
  function basePayload() {
    return buildZettelPayload(config);
  }

  async function buildIndex() {
    reports.clearApiUsage();
    await runner.runWorkflow("Building zettel index", "/api/workflows/zettel/index", basePayload(), (output) => {
      reports.updateApiUsage(output);
      runner.status.value = "Embedding index is ready";
      runner.result.value = stringify(output);
    });
  }

  async function previewInboxCandidates() {
    reports.clearApiUsage();
    await runner.runWorkflow("Previewing embedding matches", "/api/workflows/zettel/inbox-candidates", basePayload(), (output) => {
      reports.updateApiUsage(output);
      const response = output as InboxCandidatePreviewReport;
      reports.candidatePreview.value = response;
      runner.status.value = `Embedding preview completed: ${response.selected_count} source notes, ${totalCandidateCount(response)} candidates`;
      runner.result.value = stringify(output);
    });
  }

  async function rollback() {
    reports.clearApiUsage();
    await runner.runWorkflow("Rolling back zettel merge", "/api/workflows/zettel/rollback", {
      ...basePayload(),
      job_id: reports.rollbackJobID.value,
    }, (output) => {
      reports.updateApiUsage(output);
      runner.status.value = "Rollback completed";
      runner.result.value = stringify(output);
    });
  }

  async function runInboxMerge() {
    reports.clearApiUsage();
    await runner.runWorkflow("Running inbox merge", "/api/workflows/zettel/inbox-merge", basePayload(), (output) => {
      reports.updateApiUsage(output);
      const response = output as InboxMergeReport;
      reports.inboxReport.value = response;
      reports.rollbackJobID.value = response.run_id || "";
      runner.status.value = `Inbox merge completed: ${response.processed_count} processed, ${response.pending_count} pending, ${response.failed_count} failed`;
      runner.result.value = stringify(output);
    });
  }

  async function runMetadata() {
    reports.clearApiUsage();
    await runner.runWorkflow("Generating note metadata", "/api/workflows/zettel/metadata", buildZettelMetadataPayload(config), (output) => {
      reports.updateApiUsage(output);
      const response = output as MetadataReport;
      reports.metadataReport.value = response;
      runner.status.value = `Metadata completed: ${response.processed_count} updated, ${response.skipped?.length || 0} skipped, ${response.failed_count} failed`;
      runner.result.value = stringify(output);
    });
  }

  async function exportTrainingData() {
    reports.clearApiUsage();
    await runner.runWorkflow(
      "Exporting clean training data",
      "/api/workflows/zettel/training-export",
      buildZettelTrainingPayload(config),
      (output) => {
        reports.updateApiUsage(output);
        const response = output as TrainingExportReport;
        reports.trainingReport.value = response;
        runner.status.value = [
          `Training export completed: ${response.train_count} train`,
          `${response.eval_count} eval`,
          `${response.skipped_count} skipped`,
        ].join(", ");
        runner.result.value = stringify(output);
      },
    );
  }

  return {
    buildIndex,
    previewInboxCandidates,
    rollback,
    runInboxMerge,
    runMetadata,
    exportTrainingData,
  };
}

function totalCandidateCount(report: InboxCandidatePreviewReport) {
  return report.sources.reduce((sum, source) => sum + source.candidates.length, 0);
}
