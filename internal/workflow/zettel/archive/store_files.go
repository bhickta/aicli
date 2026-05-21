package archive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (s Store) jobPath(jobID string) (string, error) {
	if strings.TrimSpace(jobID) == "" {
		return "", errors.New("job id is required")
	}
	return s.vault.DataPath(s.options, "jobs", sanitizeFileName(jobID))
}

func (s Store) readManifest(base string) (archiveManifest, error) {
	var manifest archiveManifest
	data, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		return manifest, fmt.Errorf("read archive manifest: %w", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, fmt.Errorf("parse archive manifest: %w", err)
	}
	return manifest, nil
}

func (s Store) writeManifest(base string, manifest archiveManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "manifest.json"), append(data, '\n'), 0o600)
}

func (s Store) readIndex() ([]archiveIndexEntry, error) {
	path, err := s.vault.DataPath(s.options, "jobs", "index.json")
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []archiveIndexEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read archive index: %w", err)
	}
	var index []archiveIndexEntry
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("parse archive index: %w", err)
	}
	return index, nil
}

func (s Store) updateIndex(manifest archiveManifest) error {
	path, err := s.vault.DataPath(s.options, "jobs", "index.json")
	if err != nil {
		return err
	}
	index, err := s.readIndex()
	if err != nil {
		return err
	}
	sourcePaths := make([]string, 0, len(manifest.Sources))
	for _, source := range manifest.Sources {
		sourcePaths = append(sourcePaths, source.OriginalPath)
	}
	entry := archiveIndexEntry{
		JobID:       manifest.JobID,
		CreatedAt:   manifest.CreatedAt,
		Status:      manifest.Status,
		TargetPath:  manifest.Target.OriginalPath,
		SourcePaths: sourcePaths,
		AppliedAt:   manifest.AppliedAt,
		RestoredAt:  manifest.RestoredAt,
	}
	replaced := false
	for i := range index {
		if index[i].JobID == manifest.JobID {
			index[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		index = append(index, entry)
	}
	sort.Slice(index, func(i, j int) bool {
		return index[i].CreatedAt.Before(index[j].CreatedAt)
	})
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create archive index folder: %w", err)
	}
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "note"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return replacer.Replace(name)
}
