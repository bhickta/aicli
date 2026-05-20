package systemresources

import (
	"strconv"
	"strings"
)

func parseInt(value string) int {
	out, _ := strconv.Atoi(strings.TrimSpace(value))
	return out
}

func parseFloat(value string) float64 {
	out, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return out
}

func clamp(value, low, high int) int {
	if high < low {
		high = low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
