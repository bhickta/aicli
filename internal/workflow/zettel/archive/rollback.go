package archive

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (s Store) Rollback(jobID string) (string, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		var err error
		jobID, err = s.latestInboxRun()
		if err != nil {
			return "", err
		}
	}
	return s.rollbackInboxRun(jobID)
}

func (s Store) latestInboxRun() (string, error) {
	root, err := s.vault.DataPath(s.options, "inbox-runs")
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("no zettel inbox merge run found")
	}
	if err != nil {
		return "", fmt.Errorf("read inbox runs: %w", err)
	}
	var latestID string
	var latestTime time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := s.readInboxManifest(filepath.Join(root, entry.Name()))
		if err != nil || manifest.RestoredAt != nil {
			continue
		}
		if !inboxRunHasRestorableWrites(manifest) {
			continue
		}
		created := manifest.CreatedAt
		if manifest.CompletedAt != nil {
			created = *manifest.CompletedAt
		}
		if latestID == "" || created.After(latestTime) {
			latestID = manifest.RunID
			latestTime = created
		}
	}
	if latestID == "" {
		return "", errors.New("no restorable zettel inbox merge run found")
	}
	return latestID, nil
}

func inboxRunHasRestorableWrites(manifest inboxRunManifest) bool {
	for _, item := range manifest.Items {
		if item.Status != "processed" && item.Status != "partial" {
			continue
		}
		if item.ProcessedPath != "" || len(item.Destinations) > 0 {
			return true
		}
	}
	return false
}
