package videoapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
	"github.com/bhickta/aicli/internal/systemresources"
	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/video"
)

type Handler struct {
	runtime *core.Runtime
}

func New(runtime *core.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/workflows/video/info", h.runVideoInfo)
	mux.HandleFunc("POST /api/workflows/video/compress", h.runVideoCompress)
	mux.HandleFunc("POST /api/workflows/video/course", h.runVideoCourse)
	mux.HandleFunc("POST /api/workflows/video/metadata/backup", h.runVideoMetadataBackup)
	mux.HandleFunc("POST /api/workflows/video/metadata/restore", h.runVideoMetadataRestore)
	mux.HandleFunc("POST /api/workflows/video/generate", h.runVideoGenerate)
}

func (h *Handler) runVideoInfo(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[video.InfoRequest](w, r)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "video-info", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("probing media metadata"))
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Info(ctx, req)
		return result, err
	})
}

func (h *Handler) runVideoCompress(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[video.CompressRequest](w, r)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "video-compress", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("compressing video"))
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Compress(ctx, req)
		return result, err
	})
}

func (h *Handler) runVideoCourse(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[video.CourseRequest](w, r)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "video-course", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		resources := systemresources.Collect(ctx)
		transcriptWorkers := req.TranscriptWorkers
		compressionWorkers := req.CompressionWorkers
		if transcriptWorkers <= 0 && req.Workers > 0 {
			transcriptWorkers = req.Workers
		}
		if compressionWorkers <= 0 && req.Workers > 0 {
			compressionWorkers = req.Workers
		}
		if transcriptWorkers <= 0 {
			transcriptWorkers = systemresources.DefaultTranscriptWorkers(req.WhisperModel, 6, resources)
		}
		if compressionWorkers <= 0 {
			compressionWorkers = systemresources.DefaultCompressionWorkers(6, resources)
		}
		progress(core.Indeterminate(fmt.Sprintf("processing course with Whisper model %s on %s using %d transcribe/%d compress worker(s)", displayValue(req.WhisperModel, "large-v3"), displayValue(req.WhisperDevice, "cuda"), transcriptWorkers, compressionWorkers)))
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).CourseWithProgress(ctx, req, progress)
		return result, err
	})
}

func displayValue(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (h *Handler) runVideoMetadataBackup(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[video.MetadataRequest](w, r)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "video-metadata-backup", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("backing up metadata"))
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).BackupMetadata(ctx, req)
		return result, err
	})
}

func (h *Handler) runVideoMetadataRestore(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[video.MetadataRequest](w, r)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "video-metadata-restore", req.Path, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("restoring metadata"))
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).RestoreMetadata(ctx, req)
		return result, err
	})
}

func (h *Handler) runVideoGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := core.DecodeJSON[struct {
		ProviderID string `json:"provider_id"`
		video.LLMRequest
	}](w, r)
	if !ok {
		return
	}
	p, ok := h.runtime.ProviderOrError(w, req.ProviderID)
	if !ok {
		return
	}
	h.runtime.StartJob(w, r, "video-"+req.Mode, req.Title, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress(core.Indeterminate("generating video workflow text"))
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}, p).Generate(ctx, req.LLMRequest)
		return result, err
	})
}
