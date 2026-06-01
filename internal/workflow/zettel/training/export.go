package training

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	progressmodel "github.com/bhickta/aicli/internal/progress"
	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

func (r Runner) Export(
	ctx context.Context,
	req TrainingExportRequest,
	progress ProgressFunc,
) (TrainingExportResponse, error) {
	options := model.NormalizeOptions(req.Options)
	v, err := vaultfs.New(options.VaultPath)
	if err != nil {
		return TrainingExportResponse{}, err
	}
	dataRoot, err := v.DataPath(options)
	if err != nil {
		return TrainingExportResponse{}, err
	}

	runID := fmt.Sprintf("zettel-training-export-%d", time.Now().UTC().UnixNano())
	exportPath, err := v.DataPath(options, "training-exports", runID)
	if err != nil {
		return TrainingExportResponse{}, err
	}
	if err := os.MkdirAll(exportPath, 0o755); err != nil {
		return TrainingExportResponse{}, fmt.Errorf("create training export folder: %w", err)
	}

	files, err := findInboxTrainingFiles(dataRoot)
	if err != nil {
		return TrainingExportResponse{}, err
	}

	response := TrainingExportResponse{
		RunID:             runID,
		ArchivePath:       exportPath,
		TrainPath:         filepath.Join(exportPath, "train.jsonl"),
		EvalPath:          filepath.Join(exportPath, "eval.jsonl"),
		ShareGPTTrainPath: filepath.Join(exportPath, "train.sharegpt.jsonl"),
		ShareGPTEvalPath:  filepath.Join(exportPath, "eval.sharegpt.jsonl"),
		ManifestPath:      filepath.Join(exportPath, "manifest.json"),
		SourceFiles:       files,
		Strict:            req.Strict,
		SkippedByReason:   map[string]int{},
	}
	reportProgress(progress, "scanning training archives", 0, len(files), "file")

	records, err := r.collectExamples(ctx, files, &response, progress)
	if err != nil {
		return response, err
	}
	if req.Strict {
		records = applyStrictFilters(records, &response)
	}
	trainRecords, evalRecords := splitRecords(records)

	if err := writeJSONL(response.TrainPath, trainRecords); err != nil {
		return response, err
	}
	if err := writeJSONL(response.EvalPath, evalRecords); err != nil {
		return response, err
	}
	if err := writeShareGPTJSONL(response.ShareGPTTrainPath, trainRecords); err != nil {
		return response, err
	}
	if err := writeShareGPTJSONL(response.ShareGPTEvalPath, evalRecords); err != nil {
		return response, err
	}

	response.ExportedCount = len(records)
	response.TrainCount = len(trainRecords)
	response.EvalCount = len(evalRecords)
	response.Quality = buildQualityReport(records)
	if err := writeManifest(response); err != nil {
		return response, err
	}
	reportProgress(progress, "exported clean training dataset", len(files), len(files), "file")
	return response, nil
}

func (r Runner) collectExamples(
	ctx context.Context,
	files []string,
	response *TrainingExportResponse,
	progress ProgressFunc,
) ([]exportRecord, error) {
	records := []exportRecord{}
	seen := map[string]bool{}
	for i, path := range files {
		if err := ctx.Err(); err != nil {
			return records, err
		}
		reportProgress(progress, "reading training archive", i, len(files), "file")
		err := readJSONLLines(path, func(line []byte) error {
			response.ScannedCount++
			var exchange archivepkg.LLMExchange
			if err := json.Unmarshal(line, &exchange); err != nil {
				addSkipped(response, "malformed-json")
				return nil
			}

			example, reason := cleanExchange(exchange)
			if reason != "" {
				addSkipped(response, reason)
				return nil
			}
			hash := exampleHash(example)
			if seen[hash] {
				response.DuplicateCount++
				return nil
			}
			quality := inspectExample(example)
			seen[hash] = true
			records = append(records, exportRecord{
				hash:       hash,
				systemHash: quality.systemHash,
				example:    example,
				quality:    quality,
			})
			return nil
		})
		if err != nil {
			return records, err
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].hash < records[j].hash
	})
	return records, nil
}

func applyStrictFilters(records []exportRecord, response *TrainingExportResponse) []exportRecord {
	primarySystemHash := mostCommonSystemPromptHash(records)
	filtered := make([]exportRecord, 0, len(records))
	for _, record := range records {
		reason := strictSkipReason(record.quality, primarySystemHash)
		if reason != "" {
			addSkipped(response, reason)
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered
}

func addSkipped(response *TrainingExportResponse, reason string) {
	response.SkippedCount++
	response.SkippedByReason[reason]++
}

func reportProgress(progress ProgressFunc, stage string, completed int, total int, label string) {
	if progress == nil {
		return
	}
	if total <= 0 {
		progress(progressmodel.Indeterminate(stage))
		return
	}
	progress(progressmodel.Units(stage, completed, total, label))
}
