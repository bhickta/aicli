package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/storage"
)

type jobLister interface {
	ListJobsFiltered(ctx context.Context, opts storage.JobListOptions) ([]storage.Job, error)
}

type finishedJobDeleter interface {
	DeleteFinishedJobs(ctx context.Context) (int64, error)
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	opts := storage.JobListOptions{
		Status: r.URL.Query().Get("status"),
		Limit:  readJobLimit(r),
	}
	jobs, err := s.listJobsWithOptions(r, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (s *Server) listJobsWithOptions(r *http.Request, opts storage.JobListOptions) ([]storage.Job, error) {
	if opts.Status == "" {
		opts.Status = "recent"
	}
	if lister, ok := s.deps.Store.(jobLister); ok {
		return lister.ListJobsFiltered(r.Context(), opts)
	}
	jobs, err := s.deps.Store.ListJobs(r.Context())
	if err != nil {
		return nil, err
	}
	return filterJobs(jobs, opts), nil
}

func readJobLimit(r *http.Request) int {
	value := r.URL.Query().Get("limit")
	if value == "" {
		return 20
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return 20
	}
	if limit < 1 {
		return 20
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func filterJobs(jobs []storage.Job, opts storage.JobListOptions) []storage.Job {
	filtered := make([]storage.Job, 0, len(jobs))
	for _, job := range jobs {
		if jobMatchesStatus(job, opts.Status) {
			filtered = append(filtered, job)
		}
	}
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func jobMatchesStatus(job storage.Job, status string) bool {
	switch status {
	case "", "recent", "all":
		return true
	case "finished":
		return job.Status == storage.JobStatusCompleted || job.Status == storage.JobStatusFailed || job.Status == storage.JobStatusCancelled
	default:
		return job.Status == status
	}
}

func (s *Server) clearJobs(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("scope") != "finished" {
		writeError(w, http.StatusBadRequest, errors.New("scope=finished is required"))
		return
	}
	deleter, ok := s.deps.Store.(finishedJobDeleter)
	if !ok {
		writeError(w, http.StatusNotImplemented, errors.New("job cleanup is not supported by this store"))
		return
	}
	deleted, err := deleter.DeleteFinishedJobs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted})
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	var job storage.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if job.ID == "" || job.Type == "" {
		writeError(w, http.StatusBadRequest, errors.New("job id and type are required"))
		return
	}
	if job.Status == "" {
		job.Status = "queued"
	}
	if err := s.deps.Store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
	job, err := s.deps.Store.GetJob(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}
