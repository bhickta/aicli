package inbox

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	archivepkg "github.com/bhickta/aicli/internal/workflow/zettel/archive"
	"github.com/bhickta/aicli/internal/workflow/zettel/notetext"
)

func findExactDestinationDuplicate(v vault, options Options, sourcePath string, sourceContent string) (string, bool, error) {
	if strings.TrimSpace(sourceContent) == "" {
		return "", false, nil
	}
	notes, err := v.ScanNotes(options)
	if err != nil {
		return "", false, err
	}
	sort.Strings(notes)
	sourceHash := notetext.HashText(sourceContent)
	sourceBase := filepath.Base(sourcePath)
	fallback := ""
	for _, notePath := range notes {
		abs, err := v.NotePath(notePath, options)
		if err != nil {
			return "", false, err
		}
		contentBytes, err := os.ReadFile(abs)
		if err != nil {
			return "", false, fmt.Errorf("read destination note: %w", err)
		}
		content := string(contentBytes)
		if notetext.HashText(content) != sourceHash || content != sourceContent {
			continue
		}
		if filepath.Base(notePath) == sourceBase {
			return notePath, true, nil
		}
		if fallback == "" {
			fallback = notePath
		}
	}
	if fallback != "" {
		return fallback, true, nil
	}
	return "", false, nil
}

func processExactDuplicateInboxSource(v vault, archive archivepkg.Store, runID string, options Options, sourcePath string, sourceContent string, destinationPath string) (InboxSourceResult, error) {
	result := InboxSourceResult{
		SourcePath:       sourcePath,
		Status:           inboxStatusProcessed,
		DestinationPaths: []string{destinationPath},
		DedupedCount:     1,
		Claims: []InboxClaim{{
			ID:     "source-note",
			Text:   "Entire source note is byte-identical to an existing destination note.",
			Source: sourcePath,
		}},
		Ledger: []InboxClaimLedger{{
			ClaimID:         "source-note",
			Status:          claimStatusDeduped,
			DestinationPath: destinationPath,
			Evidence:        "Inbox source content exactly matches the destination note.",
			Reason:          "No merge needed because the destination already contains the whole source note byte-for-byte.",
		}},
	}
	if _, err := archive.WriteInboxItem(runID, result, sourceContent, nil, nil); err != nil {
		return result, err
	}
	processedPath, err := moveInboxSourceToProcessed(v, options, sourcePath)
	if err != nil {
		return result, err
	}
	result.ProcessedPath = processedPath
	if err := archive.UpdateInboxItemProcessedPath(runID, sourcePath, processedPath); err != nil {
		return result, err
	}
	return result, nil
}
