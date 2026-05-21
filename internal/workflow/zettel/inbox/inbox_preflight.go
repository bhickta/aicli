package inbox

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
)

func inboxSelectionNeedsProviderPreflight(v vault, options Options, sourceNotes []string) (bool, error) {
	for _, sourcePath := range sourceNotes {
		sourceAbs, err := v.Abs(sourcePath)
		if err != nil {
			return false, err
		}
		sourceBytes, err := os.ReadFile(sourceAbs)
		if err != nil {
			return false, fmt.Errorf("read inbox source: %w", err)
		}
		if _, ok, err := findExactDestinationDuplicate(v, options, sourcePath, string(sourceBytes)); err != nil {
			return false, err
		} else if !ok {
			return true, nil
		}
	}
	return false, nil
}

func (r Runner) preflightInboxMerge(ctx context.Context, v vault, options Options) error {
	index := indexer.New(v, options, r.embeddingProvider)
	cachedItems, err := index.CachedItemCount()
	if err != nil {
		return fmt.Errorf("load zettelkasten embedding index: %w", err)
	}
	if cachedItems == 0 {
		return errors.New("zettelkasten embedding index is empty; run Build Index after selecting the destination notes folder")
	}
	return nil
}
