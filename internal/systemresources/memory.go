package systemresources

import (
	"bufio"
	"bytes"
	"os"
	"strconv"
	"strings"
)

func readMemory() MemoryUsage {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemoryUsage{}
	}

	values := map[string]uint64{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err == nil {
			values[key] = value * 1024
		}
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	if available == 0 {
		available = values["MemFree"] + values["Buffers"] + values["Cached"]
	}

	used := uint64(0)
	usage := 0.0
	if total > available {
		used = total - available
	}
	if total > 0 {
		usage = 100 * float64(used) / float64(total)
	}

	return MemoryUsage{
		TotalBytes:     total,
		AvailableBytes: available,
		UsedBytes:      used,
		UsagePercent:   usage,
	}
}
