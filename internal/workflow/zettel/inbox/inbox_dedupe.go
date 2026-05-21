package inbox

import "strings"

func materializeDedupedDestinations(v vault, options Options, ledger []InboxClaimLedger, before map[string]string, after map[string]string, paths *[]string) error {
	for _, item := range ledger {
		if item.Status != claimStatusDeduped || strings.TrimSpace(item.DestinationPath) == "" {
			continue
		}
		if _, ok := before[item.DestinationPath]; ok {
			continue
		}
		content, err := readDestinationNote(v, options, item.DestinationPath)
		if err != nil {
			return err
		}
		before[item.DestinationPath] = content
		after[item.DestinationPath] = content
		*paths = appendUniquePath(*paths, item.DestinationPath)
	}
	return nil
}
