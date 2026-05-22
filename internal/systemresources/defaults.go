package systemresources

import (
	"runtime"
	"strings"
)

func Defaults(snapshot Snapshot) WorkerDefaults {
	return WorkerDefaults{
		VideoTranscriptWorkers:  DefaultTranscriptWorkers("large-v3", 6, snapshot),
		VideoCompressionWorkers: DefaultCompressionWorkers(6, snapshot),
		PDFRenderWorkers:        DefaultPDFRenderWorkers(12, snapshot),
		OCRWorkers:              DefaultOCRWorkers(12, snapshot),
		ZettelReadWorkers:       DefaultZettelReadWorkers(snapshot),
		EmbeddingBatchSize:      128,
		EmbeddingWorkers:        DefaultEmbeddingWorkers(snapshot),
	}
}

func DefaultEmbeddingWorkers(snapshot Snapshot) int {
	freeVRAM := maxFreeVRAM(snapshot)
	switch {
	case freeVRAM >= 18000:
		return 4
	case freeVRAM >= 8000:
		return 3
	case freeVRAM >= 4000:
		return 2
	default:
		return 2
	}
}

func DefaultTranscriptWorkers(model string, jobs int, snapshot Snapshot) int {
	if jobs <= 1 {
		return 1
	}

	model = strings.ToLower(model)
	limit := transcriptWorkerLimit(model, maxFreeVRAM(snapshot))
	return clamp(limit, 1, min(jobs, 8))
}

func transcriptWorkerLimit(model string, freeVRAM int) int {
	switch {
	case strings.Contains(model, "tiny") || strings.Contains(model, "base"):
		if freeVRAM > 0 && freeVRAM < 1024 {
			return 1
		}
		return 4
	case strings.Contains(model, "small"):
		if freeVRAM > 0 && freeVRAM < 2500 {
			return 1
		}
		return 4
	case strings.Contains(model, "medium"):
		if freeVRAM > 0 && freeVRAM < 4500 {
			return 1
		}
		return 3
	case strings.Contains(model, "turbo"):
		if freeVRAM > 0 && freeVRAM < 3000 {
			return 1
		}
		if freeVRAM >= 3000 && freeVRAM < 6000 {
			return 2
		}
		return 3
	default:
		if freeVRAM > 0 && freeVRAM < 9000 {
			return 1
		}
		if freeVRAM >= 18000 {
			return 3
		}
		return 2
	}
}

func DefaultCompressionWorkers(jobs int, snapshot Snapshot) int {
	cpus := cpuCount(snapshot)
	limit := cpus / 3
	if limit < 2 {
		limit = 2
	}
	return clamp(limit, 1, min(jobs, 6))
}

func DefaultPDFRenderWorkers(jobs int, snapshot Snapshot) int {
	cpus := cpuCount(snapshot)
	limit := cpus / 2
	if limit < 2 {
		limit = 2
	}
	return clamp(limit, 1, min(jobs, cpus))
}

func DefaultOCRWorkers(jobs int, snapshot Snapshot) int {
	cpus := cpuCount(snapshot)
	limit := cpus / 4
	if limit < 1 {
		limit = 1
	}
	if limit > 4 {
		limit = 4
	}
	return clamp(limit, 1, jobs)
}

func DefaultZettelReadWorkers(snapshot Snapshot) int {
	cpus := cpuCount(snapshot)
	return clamp(cpus, 1, 8)
}

func cpuCount(snapshot Snapshot) int {
	if snapshot.CPU.LogicalCores > 0 {
		return snapshot.CPU.LogicalCores
	}
	return runtime.NumCPU()
}

func maxFreeVRAM(snapshot Snapshot) int {
	maxFree := 0
	for _, gpu := range snapshot.GPUs {
		if gpu.MemoryFreeMB > maxFree {
			maxFree = gpu.MemoryFreeMB
		}
	}
	return maxFree
}
