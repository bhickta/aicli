package systemresources

import (
	"context"
	"os/exec"
	"strings"
)

func readGPUs(ctx context.Context) []GPUUsage {
	out, err := exec.CommandContext(
		ctx,
		"nvidia-smi",
		"--query-gpu=name,memory.total,memory.used,utilization.gpu,utilization.memory",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	gpus := make([]GPUUsage, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}

		total := parseInt(fields[1])
		used := parseInt(fields[2])
		gpus = append(gpus, GPUUsage{
			Name:                     strings.TrimSpace(fields[0]),
			MemoryTotalMB:            total,
			MemoryUsedMB:             used,
			MemoryFreeMB:             max(0, total-used),
			UtilizationPercent:       parseFloat(fields[3]),
			MemoryUtilizationPercent: parseFloat(fields[4]),
		})
	}
	return gpus
}
