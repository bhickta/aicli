package archive

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (s Store) Rollback(jobID string) (string, error) {
	if jobID != "" {
		restored, err := s.rollbackInboxRun(jobID)
		if err == nil {
			return restored, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	if jobID == "" {
		var err error
		jobID, err = s.latestApplied()
		if err != nil {
			return "", err
		}
	}
	base, err := s.jobPath(jobID)
	if err != nil {
		return "", err
	}
	manifest, err := s.readManifest(base)
	if err != nil {
		return "", err
	}
	if err := s.restoreEntry(base, manifest.Target); err != nil {
		return "", err
	}
	for _, source := range manifest.Sources {
		if err := s.restoreEntry(base, source); err != nil {
			return "", err
		}
	}
	now := time.Now().UTC()
	manifest.Status = "restored"
	manifest.RestoredAt = &now
	if err := s.writeManifest(base, manifest); err != nil {
		return "", err
	}
	if err := s.updateIndex(manifest); err != nil {
		return "", err
	}
	return jobID, nil
}

func (s Store) restoreEntry(base string, entry archiveEntry) error {
	content, err := os.ReadFile(filepath.Join(base, entry.ArchivePath))
	if err != nil {
		return fmt.Errorf("read archive entry: %w", err)
	}
	abs, err := s.vault.Abs(entry.OriginalPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create restore folder: %w", err)
	}
	return os.WriteFile(abs, content, 0o600)
}

func (s Store) latestApplied() (string, error) {
	index, err := s.readIndex()
	if err != nil {
		return "", err
	}
	for i := len(index) - 1; i >= 0; i-- {
		if index[i].Status == "applied" {
			return index[i].JobID, nil
		}
	}
	return "", errors.New("no applied zettel merge job found")
}
