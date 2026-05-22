package systemresources

import (
	"context"
	"runtime"
	"time"
)

type Snapshot struct {
	CollectedAt time.Time      `json:"collected_at"`
	CPU         CPUUsage       `json:"cpu"`
	RAM         MemoryUsage    `json:"ram"`
	GPUs        []GPUUsage     `json:"gpus"`
	Defaults    WorkerDefaults `json:"defaults"`
}

type CPUUsage struct {
	LogicalCores int     `json:"logical_cores"`
	UsagePercent float64 `json:"usage_percent"`
	Load1        float64 `json:"load_1"`
	Load5        float64 `json:"load_5"`
	Load15       float64 `json:"load_15"`
}

type MemoryUsage struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	UsagePercent   float64 `json:"usage_percent"`
}

type GPUUsage struct {
	Name                     string  `json:"name"`
	MemoryTotalMB            int     `json:"memory_total_mb"`
	MemoryUsedMB             int     `json:"memory_used_mb"`
	MemoryFreeMB             int     `json:"memory_free_mb"`
	UtilizationPercent       float64 `json:"utilization_percent"`
	MemoryUtilizationPercent float64 `json:"memory_utilization_percent"`
}

type WorkerDefaults struct {
	VideoTranscriptWorkers  int `json:"video_transcript_workers"`
	VideoCompressionWorkers int `json:"video_compression_workers"`
	PDFRenderWorkers        int `json:"pdf_render_workers"`
	OCRWorkers              int `json:"ocr_workers"`
	ZettelReadWorkers       int `json:"zettel_read_workers"`
	EmbeddingBatchSize      int `json:"embedding_batch_size"`
	EmbeddingWorkers        int `json:"embedding_workers"`
}

func Collect(ctx context.Context) Snapshot {
	snapshot := Snapshot{
		CollectedAt: time.Now().UTC(),
		CPU: CPUUsage{
			LogicalCores: runtime.NumCPU(),
		},
		RAM:  readMemory(),
		GPUs: readGPUs(ctx),
	}
	usage := readCPUUsage(ctx)
	snapshot.CPU.UsagePercent = usage
	snapshot.CPU.Load1, snapshot.CPU.Load5, snapshot.CPU.Load15 = readLoadAverage()
	snapshot.Defaults = Defaults(snapshot)
	return snapshot
}
