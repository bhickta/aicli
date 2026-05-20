package document

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/systemresources"
)

func OCRImages(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	workers int,
) ([]OCRPage, error) {
	if vision == nil {
		return nil, errors.New("provider is required")
	}
	workers = normalizeOCRWorkers(workers, len(inputs))
	pages := make([]OCRPage, len(inputs))
	jobs := make(chan int)
	errCh := make(chan pageError, len(inputs))

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go ocrImageWorker(ctx, vision, model, inputs, prompt, pages, jobs, errCh, &wg)
	}
	for index := range inputs {
		select {
		case <-ctx.Done():
			errCh <- pageError{Name: inputs[index].Name, Err: ctx.Err()}
			pages[index] = failedOCRPage(inputs[index], ctx.Err())
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()
	close(errCh)

	failures := []pageError{}
	for err := range errCh {
		failures = append(failures, err)
	}
	if len(failures) == len(inputs) && len(inputs) > 0 {
		return pages, pageErrors(failures)
	}
	return pages, nil
}

func normalizeOCRWorkers(workers int, jobs int) int {
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

func ocrImageWorker(
	ctx context.Context,
	vision provider.Provider,
	model string,
	inputs []ImageInput,
	prompt string,
	pages []OCRPage,
	jobs <-chan int,
	errCh chan<- pageError,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for index := range jobs {
		page, err := ocrImage(ctx, vision, model, inputs[index], prompt)
		if err != nil {
			errCh <- pageError{Name: inputs[index].Name, Err: err}
			pages[index] = failedOCRPage(inputs[index], err)
			continue
		}
		pages[index] = page
	}
}

func ocrImage(ctx context.Context, vision provider.Provider, model string, input ImageInput, prompt string) (OCRPage, error) {
	data := input.Data
	if data == nil {
		fileData, err := os.ReadFile(input.Path)
		if err != nil {
			return OCRPage{}, err
		}
		data = fileData
	}
	mimeType := input.MIMEType
	if mimeType == "" {
		mimeType = "image/jpeg"
	}
	res, err := vision.Vision(ctx, provider.VisionRequest{
		Model:       model,
		Prompt:      prompt,
		Image:       data,
		MIMEType:    mimeType,
		Temperature: 0,
		MaxTokens:   2200,
	})
	if err != nil {
		return OCRPage{}, err
	}
	return OCRPage{
		Name: input.Name,
		Path: input.Path,
		Text: strings.TrimSpace(res.Content),
	}, nil
}

type pageError struct {
	Name string
	Err  error
}

type pageErrors []pageError

func (e pageErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, item := range e {
		name := item.Name
		if name == "" {
			name = "page"
		}
		parts = append(parts, name+": "+item.Err.Error())
	}
	return strings.Join(parts, "; ")
}

func failedOCRPage(input ImageInput, err error) OCRPage {
	return OCRPage{
		Name: input.Name,
		Path: input.Path,
		Text: "> OCR failed for this page: " + err.Error(),
	}
}
