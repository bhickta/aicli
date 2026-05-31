package archive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

type MetadataResponse = model.MetadataResponse
type MetadataNoteResult = model.MetadataNoteResult

type metadataRunManifest struct {
	RunID          string                `json:"run_id"`
	CreatedAt      time.Time             `json:"created_at"`
	CompletedAt    *time.Time            `json:"completed_at,omitempty"`
	Status         string                `json:"status"`
	SourceCount    int                   `json:"source_count,omitempty"`
	SelectedCount  int                   `json:"selected_count,omitempty"`
	SkippedCount   int                   `json:"skipped_count,omitempty"`
	ProcessedCount int                   `json:"processed_count,omitempty"`
	FailedCount    int                   `json:"failed_count,omitempty"`
	Limit          int                   `json:"limit,omitempty"`
	APICalls       model.APICallUsage    `json:"api_calls,omitempty"`
	LLMArchives    []string              `json:"llm_archives,omitempty"`
	Items          []metadataArchiveItem `json:"items"`
}

type metadataArchiveItem struct {
	Path            string   `json:"path"`
	Status          string   `json:"status"`
	Reason          string   `json:"reason,omitempty"`
	Title           string   `json:"title,omitempty"`
	SummaryKeywords string   `json:"summary_keywords,omitempty"`
	RecallQuestions []string `json:"recall_questions,omitempty"`
	BeforeArchive   string   `json:"before_archive,omitempty"`
	AfterArchive    string   `json:"after_archive,omitempty"`
	DiffArchive     string   `json:"diff_archive,omitempty"`
}

func (s Store) MetadataRunPath(runID string) (string, error) {
	if strings.TrimSpace(runID) == "" {
		return "", errors.New("run id is required")
	}
	return s.vault.DataPath(s.options, "metadata-runs", sanitizeFileName(runID))
}

func (s Store) WriteMetadataLLMExchange(runID string, exchange LLMExchange) (string, error) {
	base, err := s.MetadataRunPath(runID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(base, "llm"), 0o755); err != nil {
		return "", fmt.Errorf("create metadata llm archive folder: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "training"), 0o755); err != nil {
		return "", fmt.Errorf("create metadata training archive folder: %w", err)
	}
	now := time.Now().UTC()
	exchange.RunID = runID
	exchange.Workflow = "zettel-metadata"
	exchange.CreatedAt = now
	if strings.TrimSpace(exchange.Step) == "" {
		exchange.Step = "generate-metadata"
	}
	name := fmt.Sprintf(
		"%s-%s-%s.json",
		now.Format("20060102T150405.000000000Z"),
		sanitizeFileName(strings.TrimSuffix(filepath.Base(exchange.SourcePath), ".md")),
		sanitizeFileName(exchange.Step),
	)
	archivePath := filepath.Join("llm", name)
	data, err := json.MarshalIndent(exchange, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal metadata llm exchange: %w", err)
	}
	if err := os.WriteFile(filepath.Join(base, archivePath), append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("archive metadata llm exchange: %w", err)
	}
	if err := s.appendMetadataTrainingExample(base, exchange); err != nil {
		return "", err
	}
	manifest := s.readMetadataManifestOrEmpty(runID)
	if manifest.RunID == "" {
		manifest.RunID = runID
		manifest.CreatedAt = now
		manifest.Status = "running"
	}
	manifest.LLMArchives = append(manifest.LLMArchives, archivePath)
	if err := s.writeMetadataManifest(base, manifest); err != nil {
		return "", err
	}
	return archivePath, nil
}

func (s Store) appendMetadataTrainingExample(base string, exchange LLMExchange) error {
	path := filepath.Join(base, "training", "zettel-metadata-chat.jsonl")
	data, err := json.Marshal(exchange)
	if err != nil {
		return fmt.Errorf("marshal metadata training example: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open metadata training examples: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write metadata training example: %w", err)
	}
	return nil
}

func (s Store) WriteMetadataItem(runID string, result MetadataNoteResult, before string, after string) (string, error) {
	base, err := s.MetadataRunPath(runID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(base, "notes"), 0o755); err != nil {
		return "", fmt.Errorf("create metadata note archive folder: %w", err)
	}

	index := len(s.readMetadataManifestOrEmpty(runID).Items) + 1
	prefix := fmt.Sprintf("notes/note-%03d-%s", index, sanitizeFileName(strings.TrimSuffix(filepath.Base(result.Path), ".md")))
	beforeArchive := prefix + "-before.md"
	if err := os.WriteFile(filepath.Join(base, beforeArchive), []byte(before), 0o600); err != nil {
		return "", fmt.Errorf("archive metadata note before: %w", err)
	}

	item := metadataArchiveItem{
		Path:            result.Path,
		Status:          result.Status,
		Reason:          result.Reason,
		Title:           result.Title,
		SummaryKeywords: result.SummaryKeywords,
		RecallQuestions: result.RecallQuestions,
		BeforeArchive:   beforeArchive,
	}
	if strings.TrimSpace(after) != "" && after != before {
		afterArchive := prefix + "-after.md"
		diffArchive := prefix + ".diff"
		if err := os.WriteFile(filepath.Join(base, afterArchive), []byte(after), 0o600); err != nil {
			return "", fmt.Errorf("archive metadata note after: %w", err)
		}
		if result.Diff != nil {
			if err := os.WriteFile(filepath.Join(base, diffArchive), []byte(result.Diff.Diff), 0o600); err != nil {
				return "", fmt.Errorf("archive metadata note diff: %w", err)
			}
			item.DiffArchive = diffArchive
		}
		item.AfterArchive = afterArchive
	}

	manifest := s.readMetadataManifestOrEmpty(runID)
	if manifest.RunID == "" {
		manifest.RunID = runID
		manifest.CreatedAt = time.Now().UTC()
	}
	manifest.Status = "running"
	manifest.Items = append(manifest.Items, item)
	if err := s.writeMetadataManifest(base, manifest); err != nil {
		return "", err
	}
	return base, nil
}

func (s Store) FinalizeMetadataRun(runID string, response MetadataResponse) error {
	base, err := s.MetadataRunPath(runID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("create metadata run folder: %w", err)
	}
	manifest := s.readMetadataManifestOrEmpty(runID)
	if manifest.RunID == "" {
		manifest.RunID = runID
		manifest.CreatedAt = time.Now().UTC()
	}
	now := time.Now().UTC()
	manifest.CompletedAt = &now
	manifest.Status = metadataRunStatus(response)
	manifest.SourceCount = response.SourceCount
	manifest.SelectedCount = response.SelectedCount
	manifest.SkippedCount = response.SkippedCount
	manifest.ProcessedCount = response.ProcessedCount
	manifest.FailedCount = response.FailedCount
	manifest.Limit = response.Limit
	manifest.APICalls = response.APICalls
	return s.writeMetadataManifest(base, manifest)
}

func (s Store) UpdateMetadataRunAPICalls(runID string, usage model.APICallUsage) error {
	base, err := s.MetadataRunPath(runID)
	if err != nil {
		return err
	}
	manifest, err := s.readMetadataManifest(base)
	if err != nil {
		return err
	}
	manifest.APICalls = usage
	return s.writeMetadataManifest(base, manifest)
}

func metadataRunStatus(response MetadataResponse) string {
	if response.FailedCount > 0 {
		if response.ProcessedCount > 0 || len(response.Skipped) > 0 {
			return "partial"
		}
		return "failed"
	}
	return "completed"
}

func (s Store) readMetadataManifestOrEmpty(runID string) metadataRunManifest {
	base, err := s.MetadataRunPath(runID)
	if err != nil {
		return metadataRunManifest{}
	}
	manifest, err := s.readMetadataManifest(base)
	if err != nil {
		return metadataRunManifest{}
	}
	return manifest
}

func (s Store) readMetadataManifest(base string) (metadataRunManifest, error) {
	var manifest metadataRunManifest
	data, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parse metadata run manifest: %w", err)
	}
	return manifest, nil
}

func (s Store) writeMetadataManifest(base string, manifest metadataRunManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "manifest.json"), append(data, '\n'), 0o600)
}
