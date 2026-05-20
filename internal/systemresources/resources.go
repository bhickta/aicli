package systemresources

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
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

func Defaults(snapshot Snapshot) WorkerDefaults {
	return WorkerDefaults{
		VideoTranscriptWorkers:  DefaultTranscriptWorkers("large-v3", 6, snapshot),
		VideoCompressionWorkers: DefaultCompressionWorkers(6, snapshot),
		PDFRenderWorkers:        DefaultPDFRenderWorkers(12, snapshot),
		OCRWorkers:              DefaultOCRWorkers(12, snapshot),
		ZettelReadWorkers:       DefaultZettelReadWorkers(snapshot),
		EmbeddingBatchSize:      64,
	}
}

func DefaultTranscriptWorkers(model string, jobs int, snapshot Snapshot) int {
	if jobs <= 1 {
		return 1
	}
	model = strings.ToLower(model)
	limit := 2
	freeVRAM := maxFreeVRAM(snapshot)
	switch {
	case strings.Contains(model, "tiny") || strings.Contains(model, "base"):
		limit = 4
		if freeVRAM > 0 && freeVRAM < 1024 {
			limit = 1
		}
	case strings.Contains(model, "small"):
		limit = 4
		if freeVRAM > 0 && freeVRAM < 2500 {
			limit = 1
		}
	case strings.Contains(model, "medium"):
		limit = 3
		if freeVRAM > 0 && freeVRAM < 4500 {
			limit = 1
		}
	case strings.Contains(model, "turbo"):
		limit = 3
		if freeVRAM > 0 && freeVRAM < 3000 {
			limit = 1
		}
		if freeVRAM >= 3000 && freeVRAM < 6000 {
			limit = 2
		}
	default:
		limit = 2
		if freeVRAM >= 18000 {
			limit = 3
		}
		if freeVRAM > 0 && freeVRAM < 9000 {
			limit = 1
		}
	}
	return clamp(limit, 1, min(jobs, 8))
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

type cpuSample struct {
	idle  uint64
	total uint64
}

func readCPUUsage(ctx context.Context) float64 {
	first, ok := readCPUSample()
	if !ok {
		return 0
	}
	timer := time.NewTimer(120 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return 0
	case <-timer.C:
	}
	second, ok := readCPUSample()
	if !ok || second.total <= first.total {
		return 0
	}
	totalDelta := second.total - first.total
	idleDelta := second.idle - first.idle
	if totalDelta == 0 || idleDelta > totalDelta {
		return 0
	}
	return 100 * float64(totalDelta-idleDelta) / float64(totalDelta)
}

func readCPUSample() (cpuSample, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuSample{}, false
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() {
		return cpuSample{}, false
	}
	fields := strings.Fields(scanner.Text())
	if len(fields) < 8 || fields[0] != "cpu" {
		return cpuSample{}, false
	}
	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuSample{}, false
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return cpuSample{idle: idle, total: total}, true
}

func readLoadAverage() (float64, float64, float64) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, 0
	}
	return parseFloat(fields[0]), parseFloat(fields[1]), parseFloat(fields[2])
}

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
	return MemoryUsage{TotalBytes: total, AvailableBytes: available, UsedBytes: used, UsagePercent: usage}
}

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
