package archive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

type InboxMergeResponse = model.InboxMergeResponse
type InboxSourceResult = model.InboxSourceResult
type InboxClaim = model.InboxClaim
type InboxClaimLedger = model.InboxClaimLedger

type inboxRunManifest struct {
	RunID          string             `json:"run_id"`
	CreatedAt      time.Time          `json:"created_at"`
	CompletedAt    *time.Time         `json:"completed_at,omitempty"`
	Status         string             `json:"status"`
	SourceCount    int                `json:"source_count,omitempty"`
	SelectedCount  int                `json:"selected_count,omitempty"`
	SkippedCount   int                `json:"skipped_count,omitempty"`
	ProcessedCount int                `json:"processed_count,omitempty"`
	PendingCount   int                `json:"pending_count,omitempty"`
	FailedCount    int                `json:"failed_count,omitempty"`
	Limit          int                `json:"limit,omitempty"`
	APICalls       model.APICallUsage `json:"api_calls,omitempty"`
	LLMArchives    []string           `json:"llm_archives,omitempty"`
	Items          []inboxArchiveItem `json:"items"`
	RestoredAt     *time.Time         `json:"restored_at,omitempty"`
}

type inboxArchiveItem struct {
	SourcePath    string                    `json:"source_path"`
	Status        string                    `json:"status"`
	ProcessedPath string                    `json:"processed_path,omitempty"`
	SourceArchive string                    `json:"source_archive"`
	Destinations  []inboxArchiveDestination `json:"destinations,omitempty"`
	Ledger        []InboxClaimLedger        `json:"ledger,omitempty"`
	Claims        []InboxClaim              `json:"claims,omitempty"`
	Reason        string                    `json:"reason,omitempty"`
}

type inboxArchiveDestination struct {
	Path          string `json:"path"`
	BeforeArchive string `json:"before_archive"`
	AfterArchive  string `json:"after_archive"`
	DiffArchive   string `json:"diff_archive"`
	Created       bool   `json:"created,omitempty"`
}

type LLMExchange struct {
	RunID        string                `json:"run_id"`
	Workflow     string                `json:"workflow"`
	Step         string                `json:"step"`
	SourcePath   string                `json:"source_path,omitempty"`
	ProviderID   string                `json:"provider_id,omitempty"`
	Model        string                `json:"model,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
	Request      provider.ChatRequest  `json:"request"`
	Response     provider.ChatResponse `json:"response"`
	Error        string                `json:"error,omitempty"`
	ParsedFormat string                `json:"parsed_format,omitempty"`
}

func (s Store) InboxRunPath(runID string) (string, error) {
	if strings.TrimSpace(runID) == "" {
		return "", errors.New("run id is required")
	}
	return s.vault.DataPath(s.options, "inbox-runs", sanitizeFileName(runID))
}

func (s Store) WriteInboxLLMExchange(runID string, exchange LLMExchange) (string, error) {
	base, err := s.InboxRunPath(runID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(base, "llm"), 0o755); err != nil {
		return "", fmt.Errorf("create inbox llm archive folder: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "training"), 0o755); err != nil {
		return "", fmt.Errorf("create inbox training archive folder: %w", err)
	}
	now := time.Now().UTC()
	exchange.RunID = runID
	exchange.Workflow = "zettel-inbox-merge"
	exchange.CreatedAt = now
	if strings.TrimSpace(exchange.Step) == "" {
		exchange.Step = "chat"
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
		return "", fmt.Errorf("marshal inbox llm exchange: %w", err)
	}
	if err := os.WriteFile(filepath.Join(base, archivePath), append(data, '\n'), 0o600); err != nil {
		return "", fmt.Errorf("archive inbox llm exchange: %w", err)
	}
	if err := s.appendInboxTrainingExample(base, exchange); err != nil {
		return "", err
	}
	manifest := s.readInboxManifestOrEmpty(runID)
	if manifest.RunID == "" {
		manifest.RunID = runID
		manifest.CreatedAt = now
		manifest.Status = "running"
	}
	manifest.LLMArchives = append(manifest.LLMArchives, archivePath)
	if err := s.writeInboxManifest(base, manifest); err != nil {
		return "", err
	}
	return archivePath, nil
}

func (s Store) appendInboxTrainingExample(base string, exchange LLMExchange) error {
	path := filepath.Join(base, "training", "zettel-inbox-chat.jsonl")
	data, err := json.Marshal(exchange)
	if err != nil {
		return fmt.Errorf("marshal inbox training example: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open inbox training examples: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write inbox training example: %w", err)
	}
	return nil
}

func (s Store) WriteInboxItem(runID string, result InboxSourceResult, sourceContent string, destinationBefore map[string]string, destinationAfter map[string]string) (string, error) {
	base, err := s.InboxRunPath(runID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(base, "sources"), 0o755); err != nil {
		return "", fmt.Errorf("create inbox source archive folder: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "destinations"), 0o755); err != nil {
		return "", fmt.Errorf("create inbox destination archive folder: %w", err)
	}

	index := len(s.readInboxManifestOrEmpty(runID).Items) + 1
	sourceArchive := fmt.Sprintf("sources/source-%03d-%s.md", index, sanitizeFileName(strings.TrimSuffix(filepath.Base(result.SourcePath), ".md")))
	if err := os.WriteFile(filepath.Join(base, sourceArchive), []byte(sourceContent), 0o600); err != nil {
		return "", fmt.Errorf("archive inbox source: %w", err)
	}

	destinations := make([]inboxArchiveDestination, 0, len(result.Diffs))
	for i, diff := range result.Diffs {
		prefix := fmt.Sprintf("destinations/source-%03d-dest-%03d-%s", index, i+1, sanitizeFileName(strings.TrimSuffix(filepath.Base(diff.Path), ".md")))
		beforeArchive := prefix + "-before.md"
		afterArchive := prefix + "-after.md"
		diffArchive := prefix + ".diff"
		if err := os.WriteFile(filepath.Join(base, beforeArchive), []byte(destinationBefore[diff.Path]), 0o600); err != nil {
			return "", fmt.Errorf("archive destination before: %w", err)
		}
		if err := os.WriteFile(filepath.Join(base, afterArchive), []byte(destinationAfter[diff.Path]), 0o600); err != nil {
			return "", fmt.Errorf("archive destination after: %w", err)
		}
		if err := os.WriteFile(filepath.Join(base, diffArchive), []byte(diff.Diff), 0o600); err != nil {
			return "", fmt.Errorf("archive destination diff: %w", err)
		}
		destinations = append(destinations, inboxArchiveDestination{
			Path:          diff.Path,
			BeforeArchive: beforeArchive,
			AfterArchive:  afterArchive,
			DiffArchive:   diffArchive,
			Created:       diff.Created,
		})
	}

	manifest := s.readInboxManifestOrEmpty(runID)
	if manifest.RunID == "" {
		manifest.RunID = runID
		manifest.CreatedAt = time.Now().UTC()
	}
	manifest.Status = "running"
	manifest.Items = append(manifest.Items, inboxArchiveItem{
		SourcePath:    result.SourcePath,
		Status:        result.Status,
		ProcessedPath: result.ProcessedPath,
		SourceArchive: sourceArchive,
		Destinations:  destinations,
		Ledger:        result.Ledger,
		Claims:        result.Claims,
		Reason:        result.Reason,
	})
	if err := s.writeInboxManifest(base, manifest); err != nil {
		return "", err
	}
	return base, nil
}

func (s Store) UpdateInboxItemProcessedPath(runID string, sourcePath string, processedPath string) error {
	base, err := s.InboxRunPath(runID)
	if err != nil {
		return err
	}
	manifest, err := s.readInboxManifest(base)
	if err != nil {
		return err
	}
	for i := range manifest.Items {
		if manifest.Items[i].SourcePath == sourcePath {
			manifest.Items[i].ProcessedPath = processedPath
			break
		}
	}
	return s.writeInboxManifest(base, manifest)
}

func (s Store) InboxItemExists(runID string, sourcePath string) bool {
	manifest := s.readInboxManifestOrEmpty(runID)
	for _, item := range manifest.Items {
		if item.SourcePath == sourcePath {
			return true
		}
	}
	return false
}

func (s Store) FinalizeInboxRun(runID string, response InboxMergeResponse) error {
	base, err := s.InboxRunPath(runID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("create inbox run folder: %w", err)
	}
	manifest := s.readInboxManifestOrEmpty(runID)
	if manifest.RunID == "" {
		manifest.RunID = runID
		manifest.CreatedAt = time.Now().UTC()
	}
	now := time.Now().UTC()
	manifest.CompletedAt = &now
	manifest.Status = inboxRunStatus(response)
	manifest.SourceCount = response.SourceCount
	manifest.SelectedCount = response.SelectedCount
	manifest.SkippedCount = response.SkippedCount
	manifest.ProcessedCount = response.ProcessedCount
	manifest.PendingCount = response.PendingCount
	manifest.FailedCount = response.FailedCount
	manifest.Limit = response.Limit
	manifest.APICalls = response.APICalls
	return s.writeInboxManifest(base, manifest)
}

func (s Store) UpdateInboxRunAPICalls(runID string, usage model.APICallUsage) error {
	base, err := s.InboxRunPath(runID)
	if err != nil {
		return err
	}
	manifest, err := s.readInboxManifest(base)
	if err != nil {
		return err
	}
	manifest.APICalls = usage
	return s.writeInboxManifest(base, manifest)
}

func inboxRunStatus(response InboxMergeResponse) string {
	if response.FailedCount > 0 {
		if response.ProcessedCount > 0 || response.PendingCount > 0 {
			return "partial"
		}
		return "failed"
	}
	for _, item := range response.Pending {
		if item.Status == "partial" {
			return "partial"
		}
	}
	if response.PendingCount > 0 {
		if response.ProcessedCount > 0 {
			return "partial"
		}
		return "pending"
	}
	return "completed"
}

func (s Store) rollbackInboxRun(runID string) (string, error) {
	base, err := s.InboxRunPath(runID)
	if err != nil {
		return "", err
	}
	manifest, err := s.readInboxManifest(base)
	if err != nil {
		return "", err
	}
	for i := len(manifest.Items) - 1; i >= 0; i-- {
		item := manifest.Items[i]
		if item.Status != "processed" && item.Status != "partial" {
			continue
		}
		for _, destination := range item.Destinations {
			content, err := os.ReadFile(filepath.Join(base, destination.BeforeArchive))
			if err != nil {
				return "", fmt.Errorf("read inbox destination archive: %w", err)
			}
			abs, err := s.vault.Abs(destination.Path)
			if err != nil {
				return "", err
			}
			if destination.Created {
				if err := os.Remove(abs); err != nil && !errors.Is(err, os.ErrNotExist) {
					return "", fmt.Errorf("delete created destination: %w", err)
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return "", fmt.Errorf("create destination restore folder: %w", err)
			}
			if err := os.WriteFile(abs, content, 0o600); err != nil {
				return "", fmt.Errorf("restore destination: %w", err)
			}
		}
		sourceAbs, err := s.vault.Abs(item.SourcePath)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
			return "", fmt.Errorf("create source restore folder: %w", err)
		}
		if item.ProcessedPath != "" {
			processedAbs, err := s.vault.Abs(item.ProcessedPath)
			if err != nil {
				return "", err
			}
			if _, err := os.Stat(processedAbs); err == nil {
				if err := os.Rename(processedAbs, sourceAbs); err != nil {
					return "", fmt.Errorf("move processed source back: %w", err)
				}
				continue
			}
		}
		content, err := os.ReadFile(filepath.Join(base, item.SourceArchive))
		if err != nil {
			return "", fmt.Errorf("read inbox source archive: %w", err)
		}
		if err := os.WriteFile(sourceAbs, content, 0o600); err != nil {
			return "", fmt.Errorf("restore inbox source: %w", err)
		}
	}
	now := time.Now().UTC()
	manifest.Status = "restored"
	manifest.RestoredAt = &now
	if err := s.writeInboxManifest(base, manifest); err != nil {
		return "", err
	}
	return runID, nil
}

func (s Store) readInboxManifestOrEmpty(runID string) inboxRunManifest {
	base, err := s.InboxRunPath(runID)
	if err != nil {
		return inboxRunManifest{}
	}
	manifest, err := s.readInboxManifest(base)
	if err != nil {
		return inboxRunManifest{}
	}
	return manifest
}

func (s Store) readInboxManifest(base string) (inboxRunManifest, error) {
	var manifest inboxRunManifest
	data, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parse inbox run manifest: %w", err)
	}
	return manifest, nil
}

func (s Store) writeInboxManifest(base string, manifest inboxRunManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "manifest.json"), append(data, '\n'), 0o600)
}
