package ocr

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bhickta/aicli/internal/systemresources"
	"github.com/bhickta/aicli/internal/workflow/document"
)

func (s *Service) RunPDF(ctx context.Context, req Request) (Response, error) {
	return s.RunPDFWithProgress(ctx, req, nil)
}

func (s *Service) RunPDFWithProgress(ctx context.Context, req Request, progress func(string)) (Response, error) {
	images, cleanup, err := document.RenderPDFToImages(ctx, s.tools, s.runner, req.Path, req.DPI, req.RenderWorkers)
	if err != nil {
		return Response{}, err
	}
	defer cleanup()
	if s.artifactDir != "" {
		images, err = copyRenderedImages(s.artifactDir, images)
		if err != nil {
			return Response{}, err
		}
	}

	inputs := make([]document.ImageInput, 0, len(images))
	for i, imagePath := range images {
		inputs = append(inputs, document.ImageInput{
			Name:     "page-" + strconv.Itoa(i+1),
			Path:     imagePath,
			MIMEType: "image/jpeg",
		})
	}
	if progress != nil {
		progress(fmt.Sprintf("OCR %d page(s) with %d worker(s)", len(inputs), normalizedWorkerCount(req.Workers, len(inputs))))
	}
	pages, err := s.ocrInputs(ctx, req, inputs)
	if err != nil {
		return Response{}, err
	}
	if progress != nil {
		progress("assembling markdown")
	}
	return s.responseFromDocumentPages(pages), nil
}

func normalizedWorkerCount(workers int, jobs int) int {
	if jobs <= 1 {
		return 1
	}
	if workers < 1 {
		return systemresources.DefaultOCRWorkers(jobs, systemresources.Snapshot{})
	}
	if workers > jobs {
		return jobs
	}
	return workers
}
