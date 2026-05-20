package video

import "github.com/bhickta/aicli/internal/systemresources"

func withCourseWorkerDefaults(req CourseRequest, jobs int, resources systemresources.Snapshot) CourseRequest {
	if req.TranscriptWorkers <= 0 && req.Workers <= 0 {
		req.TranscriptWorkers = systemresources.DefaultTranscriptWorkers(req.WhisperModel, jobs, resources)
	}
	if req.CompressionWorkers <= 0 && req.Workers <= 0 {
		req.CompressionWorkers = systemresources.DefaultCompressionWorkers(jobs, resources)
	}
	return req
}
