package news

import (
	"sort"
	"strings"
)

func dedupe(items []Item) ([]Item, int) {
	seen := map[string]Item{}
	for _, item := range items {
		key := normalize(item.Title)
		if key == "" {
			key = normalize(item.Content)
		}
		if key == "" {
			continue
		}
		if existing, ok := seen[key]; ok {
			if len(item.Content) > len(existing.Content) {
				seen[key] = item
			}
			continue
		}
		seen[key] = item
	}
	out := make([]Item, 0, len(seen))
	for _, item := range seen {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Title < out[j].Title
	})
	return out, len(items) - len(out)
}

func normalize(value string) string {
	fields := strings.Fields(strings.ToLower(value))
	return strings.Join(fields, " ")
}

func cluster(items []Item, threshold float64) []Cluster {
	clusters := []Cluster{}
	used := make([]bool, len(items))
	for i, item := range items {
		if used[i] {
			continue
		}
		current := Cluster{Items: []Item{item}, Score: 1}
		used[i] = true
		for j := i + 1; j < len(items); j++ {
			if used[j] {
				continue
			}
			score := similarity(item, items[j])
			if score >= threshold {
				current.Items = append(current.Items, items[j])
				if score > current.Score {
					current.Score = score
				}
				used[j] = true
			}
		}
		clusters = append(clusters, current)
	}
	return clusters
}

func similarity(a Item, b Item) float64 {
	left := tokenSet(a.Title + " " + a.Content)
	right := tokenSet(b.Title + " " + b.Content)
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersection := 0
	for token := range left {
		if right[token] {
			intersection++
		}
	}
	union := len(left) + len(right) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenSet(value string) map[string]bool {
	words := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	out := map[string]bool{}
	for _, word := range words {
		if len(word) < 3 {
			continue
		}
		out[word] = true
	}
	return out
}
