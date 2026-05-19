package server

func (s *Server) registerWorkflowRoutes() {
	s.mux.HandleFunc("POST /api/workflows/recall/run", s.runRecall)
	s.mux.HandleFunc("POST /api/workflows/image/run", s.runImage)
	s.mux.HandleFunc("POST /api/workflows/image/rename", s.runImageRename)
	s.mux.HandleFunc("POST /api/workflows/image/prune-refs", s.runImagePruneRefs)
	s.mux.HandleFunc("POST /api/workflows/news/run", s.runNews)
	s.mux.HandleFunc("POST /api/workflows/ocr/run", s.runOCR)
	s.mux.HandleFunc("POST /api/workflows/ocr/pdf", s.runPDFOCR)
	s.mux.HandleFunc("POST /api/workflows/analyze/run", s.runAnalyze)
	s.mux.HandleFunc("POST /api/workflows/video/info", s.runVideoInfo)
	s.mux.HandleFunc("POST /api/workflows/video/compress", s.runVideoCompress)
	s.mux.HandleFunc("POST /api/workflows/video/course", s.runVideoCourse)
	s.mux.HandleFunc("POST /api/workflows/video/metadata/backup", s.runVideoMetadataBackup)
	s.mux.HandleFunc("POST /api/workflows/video/metadata/restore", s.runVideoMetadataRestore)
	s.mux.HandleFunc("POST /api/workflows/video/generate", s.runVideoGenerate)
	s.mux.HandleFunc("POST /api/workflows/audio/transcribe", s.runAudioTranscribe)
	s.mux.HandleFunc("POST /api/workflows/audio/analyze", s.runAudioAnalyze)
}
