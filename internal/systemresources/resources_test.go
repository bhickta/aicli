package systemresources

import "testing"

func TestDefaultTranscriptWorkersUsesModelAndFreeVRAM(t *testing.T) {
	snapshot := Snapshot{CPU: CPUUsage{LogicalCores: 12}, GPUs: []GPUUsage{{MemoryFreeMB: 4096}}}
	if got := DefaultTranscriptWorkers("large-v3", 6, snapshot); got != 1 {
		t.Fatalf("large-v3 workers = %d, want 1 with low free VRAM", got)
	}
	if got := DefaultTranscriptWorkers("turbo", 6, snapshot); got != 2 {
		t.Fatalf("turbo workers = %d, want 2 with 4GB free VRAM", got)
	}
}

func TestWorkerDefaultsRespectJobCount(t *testing.T) {
	snapshot := Snapshot{CPU: CPUUsage{LogicalCores: 12}, GPUs: []GPUUsage{{MemoryFreeMB: 24000}}}
	if got := DefaultCompressionWorkers(2, snapshot); got != 2 {
		t.Fatalf("compression workers = %d, want 2", got)
	}
	if got := DefaultPDFRenderWorkers(20, snapshot); got != 6 {
		t.Fatalf("pdf render workers = %d, want 6", got)
	}
	if got := DefaultOCRWorkers(20, snapshot); got != 3 {
		t.Fatalf("ocr workers = %d, want 3", got)
	}
	if got := DefaultZettelReadWorkers(snapshot); got != 8 {
		t.Fatalf("zettel workers = %d, want 8", got)
	}
}
