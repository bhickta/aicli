package zettel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type archiveStore struct {
	vault   vault
	options Options
}

type archiveManifest struct {
	JobID             string             `json:"job_id"`
	CreatedAt         time.Time          `json:"created_at"`
	Status            string             `json:"status"`
	Target            archiveEntry       `json:"target"`
	Sources           []archiveEntry     `json:"sources"`
	FinalHash         string             `json:"final_hash"`
	FinalArchivePath  string             `json:"final_archive_path"`
	SourceExtractions []SourceExtraction `json:"source_extractions"`
	Models            ProposalModels     `json:"models"`
	Coverage          CoverageReport     `json:"coverage"`
	Judge             MergeJudge         `json:"judge"`
	AppliedAt         *time.Time         `json:"applied_at,omitempty"`
	RestoredAt        *time.Time         `json:"restored_at,omitempty"`
}

type archiveEntry struct {
	OriginalPath string `json:"original_path"`
	ArchivePath  string `json:"archive_path"`
	SHA256       string `json:"sha256"`
	Bytes        int    `json:"bytes"`
}

type archiveIndexEntry struct {
	JobID       string     `json:"job_id"`
	CreatedAt   time.Time  `json:"created_at"`
	Status      string     `json:"status"`
	TargetPath  string     `json:"target_path"`
	SourcePaths []string   `json:"source_paths"`
	AppliedAt   *time.Time `json:"applied_at,omitempty"`
	RestoredAt  *time.Time `json:"restored_at,omitempty"`
}

func newArchiveStore(v vault, options Options) archiveStore {
	return archiveStore{vault: v, options: options}
}

func (s archiveStore) WriteBeforeApply(proposal Proposal, targetContent string, sourceContents map[string]string) (string, error) {
	base, err := s.jobPath(proposal.ID)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(base, "originals"), 0o755); err != nil {
		return "", fmt.Errorf("create archive originals folder: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "final"), 0o755); err != nil {
		return "", fmt.Errorf("create archive final folder: %w", err)
	}
	targetArchive := "originals/target.md"
	if err := os.WriteFile(filepath.Join(base, targetArchive), []byte(targetContent), 0o600); err != nil {
		return "", fmt.Errorf("archive target: %w", err)
	}
	sources := make([]archiveEntry, 0, len(proposal.SourceExtractions))
	for i, extraction := range proposal.SourceExtractions {
		content := sourceContents[extraction.Path]
		archivePath := fmt.Sprintf("originals/source-%03d-%s.md", i+1, sanitizeFileName(strings.TrimSuffix(filepath.Base(extraction.Path), ".md")))
		if err := os.WriteFile(filepath.Join(base, archivePath), []byte(content), 0o600); err != nil {
			return "", fmt.Errorf("archive source %s: %w", extraction.Path, err)
		}
		sources = append(sources, archiveEntry{
			OriginalPath: extraction.Path,
			ArchivePath:  archivePath,
			SHA256:       hashText(content),
			Bytes:        len([]byte(content)),
		})
	}
	finalArchive := "final/" + sanitizeFileName(strings.TrimSuffix(filepath.Base(proposal.ActivePath), ".md")) + ".md"
	if err := os.WriteFile(filepath.Join(base, finalArchive), []byte(proposal.FinalMarkdown), 0o600); err != nil {
		return "", fmt.Errorf("archive final note: %w", err)
	}
	manifest := archiveManifest{
		JobID:             proposal.ID,
		CreatedAt:         proposal.CreatedAt,
		Status:            "created",
		Target:            archiveEntry{OriginalPath: proposal.ActivePath, ArchivePath: targetArchive, SHA256: hashText(targetContent), Bytes: len([]byte(targetContent))},
		Sources:           sources,
		FinalHash:         hashText(proposal.FinalMarkdown),
		FinalArchivePath:  finalArchive,
		SourceExtractions: proposal.SourceExtractions,
		Models:            proposal.Models,
		Coverage:          proposal.Coverage,
		Judge:             proposal.Judge,
	}
	if err := s.writeManifest(base, manifest); err != nil {
		return "", err
	}
	return base, s.updateIndex(manifest)
}

func (s archiveStore) MarkApplied(jobID string) error {
	base, err := s.jobPath(jobID)
	if err != nil {
		return err
	}
	manifest, err := s.readManifest(base)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	manifest.Status = "applied"
	manifest.AppliedAt = &now
	if err := s.writeManifest(base, manifest); err != nil {
		return err
	}
	return s.updateIndex(manifest)
}

func (s archiveStore) Rollback(jobID string) (string, error) {
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

func (s archiveStore) restoreEntry(base string, entry archiveEntry) error {
	content, err := os.ReadFile(filepath.Join(base, entry.ArchivePath))
	if err != nil {
		return fmt.Errorf("read archive entry: %w", err)
	}
	abs, err := s.vault.abs(entry.OriginalPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return fmt.Errorf("create restore folder: %w", err)
	}
	return os.WriteFile(abs, content, 0o600)
}

func (s archiveStore) latestApplied() (string, error) {
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

func (s archiveStore) jobPath(jobID string) (string, error) {
	if strings.TrimSpace(jobID) == "" {
		return "", errors.New("job id is required")
	}
	return s.vault.dataPath(s.options, "jobs", sanitizeFileName(jobID))
}

func (s archiveStore) readManifest(base string) (archiveManifest, error) {
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

func (s archiveStore) writeManifest(base string, manifest archiveManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, "manifest.json"), append(data, '\n'), 0o600)
}

func (s archiveStore) readIndex() ([]archiveIndexEntry, error) {
	path, err := s.vault.dataPath(s.options, "jobs", "index.json")
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

func (s archiveStore) updateIndex(manifest archiveManifest) error {
	path, err := s.vault.dataPath(s.options, "jobs", "index.json")
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
