package zettel

import (
	"context"
	"errors"
	"fmt"
	"os"
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

func (s *Service) preflightInboxMerge(ctx context.Context, v vault, options Options) error {
	index := newEmbeddingIndex(v, options, s.embeddingProvider)
	cachedItems, err := index.CachedItemCount()
	if err != nil {
		return fmt.Errorf("load zettelkasten embedding index: %w", err)
	}
	if cachedItems == 0 {
		return errors.New("zettelkasten embedding index is empty; run Build Index after selecting the destination notes folder")
	}
	if err := index.CheckEmbeddingProvider(ctx); err != nil {
		return fmt.Errorf("embedding provider unavailable for inbox merge: %w", err)
	}
	return nil
}
