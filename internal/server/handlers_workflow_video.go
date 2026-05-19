package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bhickta/aicli/internal/tool"
	"github.com/bhickta/aicli/internal/workflow/video"
)

func (s *Server) runVideoInfo(w http.ResponseWriter, r *http.Request) {
	var req video.InfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-info", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("probing media metadata", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).Info(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runVideoCompress(w http.ResponseWriter, r *http.Request) {
	var req video.CompressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-compress", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("compressing video", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).Compress(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runVideoCourse(w http.ResponseWriter, r *http.Request) {
	var req video.CourseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-course", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("transcribing missing subtitles with Whisper", 2, 5)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).Course(ctx, req)
		progress("merging course video, subtitles, and transcript", 4, 5)
		return result, err
	})
}

func (s *Server) runVideoMetadataBackup(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-metadata-backup", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("backing up metadata", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).BackupMetadata(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runVideoMetadataRestore(w http.ResponseWriter, r *http.Request) {
	var req video.MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	job := s.newJob("video-metadata-restore", req.Path)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("restoring metadata", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}).RestoreMetadata(ctx, req)
		progress("saving result", 3, 4)
		return result, err
	})
}

func (s *Server) runVideoGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProviderID string `json:"provider_id"`
		video.LLMRequest
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	p, ok := s.providerFor(req.ProviderID)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("provider not found"))
		return
	}
	job := s.newJob("video-"+req.Mode, req.Title)
	s.startWorkflow(w, r, job, func(ctx context.Context, progress progressFunc) (any, error) {
		progress("generating video workflow text", 2, 4)
		result, err := video.New(s.deps.Settings.Tools, tool.ExecRunner{}, p).Generate(ctx, req.LLMRequest)
		progress("saving result", 3, 4)
		return result, err
	})
}
