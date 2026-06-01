package training

func mostCommonSystemPromptHash(records []exportRecord) string {
	counts := map[string]int{}
	for _, record := range records {
		counts[record.systemHash]++
	}
	var best string
	for hash, count := range counts {
		if count > counts[best] || (count == counts[best] && hash < best) {
			best = hash
		}
	}
	return best
}

func mostCommonSystemPromptCount(counts map[string]int) int {
	var best int
	for _, count := range counts {
		if count > best {
			best = count
		}
	}
	return best
}
