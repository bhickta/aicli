package zettel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
	Validation    MergeJudge                `json:"validation,omitempty"`
	Reason        string                    `json:"reason,omitempty"`
}

type inboxArchiveDestination struct {
	Path          string `json:"path"`
	BeforeArchive string `json:"before_archive"`
	AfterArchive  string `json:"after_archive"`
	DiffArchive   string `json:"diff_archive"`
}

func (s archiveStore) inboxRunPath(runID string) (string, error) {
	if strings.TrimSpace(runID) == "" {
		return "", errors.New("run id is required")
	}
	return s.vault.dataPath(s.options, "inbox-runs", sanitizeFileName(runID))
}

func (s archiveStore) writeInboxItem(runID string, result InboxSourceResult, sourceContent string, destinationBefore map[string]string, destinationAfter map[string]string) (string, error) {
	base, err := s.inboxRunPath(runID)
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
		Validation:    result.Validation,
		Reason:        result.Reason,
	})
	if err := s.writeInboxManifest(base, manifest); err != nil {
		return "", err
	}
	return base, nil
}

func (s archiveStore) updateInboxItemProcessedPath(runID string, sourcePath string, processedPath string) error {
	base, err := s.inboxRunPath(runID)
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

func (s archiveStore) finalizeInboxRun(runID string, response InboxMergeResponse) error {
	base, err := s.inboxRunPath(runID)
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
	return s.writeInboxManifest(base, manifest)
}

func inboxRunStatus(response InboxMergeResponse) string {
	if response.FailedCount > 0 {
		if response.ProcessedCount > 0 || response.PendingCount > 0 {
			return "partial"
		}
		return "failed"
	}
	if response.PendingCount > 0 {
		if response.ProcessedCount > 0 {
			return "partial"
		}
		return "pending"
	}
	return "completed"
}

func (s archiveStore) rollbackInboxRun(runID string) (string, error) {
	base, err := s.inboxRunPath(runID)
	if err != nil {
		return "", err
	}
	manifest, err := s.readInboxManifest(base)
	if err != nil {
		return "", err
	}
	for i := len(manifest.Items) - 1; i >= 0; i-- {
		item := manifest.Items[i]
		if item.Status != "processed" {
			continue
		}
		for _, destination := range item.Destinations {
			content, err := os.ReadFile(filepath.Join(base, destination.BeforeArchive))
			if err != nil {
				return "", fmt.Errorf("read inbox destination archive: %w", err)
			}
			abs, err := s.vault.abs(destination.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return "", fmt.Errorf("create destination restore folder: %w", err)
			}
			if err := os.WriteFile(abs, content, 0o600); err != nil {
				return "", fmt.Errorf("restore destination: %w", err)
			}
		}
		sourceAbs, err := s.vault.abs(item.SourcePath)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
			return "", fmt.Errorf("create source restore folder: %w", err)
		}
		if item.ProcessedPath != "" {
			processedAbs, err := s.vault.abs(item.ProcessedPath)
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

func (s archiveStore) readInboxManifestOrEmpty(runID string) inboxRunManifest {
	base, err := s.inboxRunPath(runID)
	if err != nil {
		return inboxRunManifest{}
	}
	manifest, err := s.readInboxManifest(base)
	if err != nil {
		return inboxRunManifest{}
	}
	return manifest
}

func (s archiveStore) readInboxManifest(base string) (inboxRunManifest, error) {
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

func (s archiveStore) writeInboxManifest(base string, manifest inboxRunManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "manifest.json"), append(data, '\n'), 0o600)
}
