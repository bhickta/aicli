package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

type Options = model.Options
type Proposal = model.Proposal
type SourceExtraction = model.SourceExtraction
type ProposalModels = model.ProposalModels
type CoverageReport = model.CoverageReport
type MergeJudge = model.MergeJudge

type Store struct {
	vault   vaultfs.Vault
	options model.Options
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

func NewStore(v vaultfs.Vault, options model.Options) Store {
	return Store{vault: v, options: options}
}

func (s Store) WriteBeforeApply(proposal Proposal, targetContent string, sourceContents map[string]string) (string, error) {
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
			SHA256:       notetext.HashText(content),
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
		Target:            archiveEntry{OriginalPath: proposal.ActivePath, ArchivePath: targetArchive, SHA256: notetext.HashText(targetContent), Bytes: len([]byte(targetContent))},
		Sources:           sources,
		FinalHash:         notetext.HashText(proposal.FinalMarkdown),
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

func (s Store) MarkApplied(jobID string) error {
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
