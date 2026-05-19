package videoapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/server/workflowapi/core"
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
	var req video.InfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	job := core.NewJob("video-info", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("probing media metadata", 2, 4)
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Info(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (h *Handler) runVideoCompress(w http.ResponseWriter, r *http.Request) {
	var req video.CompressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	job := core.NewJob("video-compress", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("compressing video", 2, 4)
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Compress(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (h *Handler) runVideoCourse(w http.ResponseWriter, r *http.Request) {
	var req video.CourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	job := core.NewJob("video-course", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("transcribing missing subtitles with Whisper", 2, 5)
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).Course(ctx, req)
		progress("merging course video, subtitles, and transcript", 4, 5)
		return result, err
	})
}

func (h *Handler) runVideoMetadataBackup(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	job := core.NewJob("video-metadata-backup", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("backing up metadata", 2, 4)
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).BackupMetadata(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (h *Handler) runVideoMetadataRestore(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	job := core.NewJob("video-metadata-restore", req.Path)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("restoring metadata", 2, 4)
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}).RestoreMetadata(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (h *Handler) runVideoGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		video.LLMRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		core.WriteError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := h.runtime.ProviderFor(req.ProviderID)
	if !ok {
		core.WriteError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := core.NewJob("video-"+req.Mode, req.Title)
	h.runtime.StartWorkflow(w, r, job, func(ctx context.Context, progress core.ProgressFunc) (any, error) {
		progress("generating video workflow text", 2, 4)
		result, err := video.New(h.runtime.Settings().Tools, tool.ExecRunner{}, p).Generate(ctx, req.LLMRequest)
		progress("saving result", 3, 4)
		return result, err
	})
}
