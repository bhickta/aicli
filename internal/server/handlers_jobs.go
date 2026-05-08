package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bhickta/aicli/internal/storage"
)

func (s *Server) newJob(jobType string, input string) storage.Job {
	now := time.Now().UTC()
	return storage.Job{
		ID:          fmt.Sprintf("%s-%d", jobType, time.Now().UTC().UnixNano()),
		Type:        jobType,
		Status:      "running",
		Stage:       "queued",
		Progress:    0,
		CurrentStep: 0,
		TotalSteps:  4,
		Input:       input,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (s *Server) listJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.deps.Store.ListJobs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
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
