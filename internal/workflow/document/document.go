package document

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/tool"
)

type OCRPage struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Text string `json:"text"`
}

type ImageInput struct {
	Name     string
	Path     string
	Data     []byte
	MIMEType string
}

func RenderPDFToImages(ctx context.Context, tools config.ToolConfig, runner tool.Runner, pdfPath string, dpi int) ([]string, func(), error) {
	if strings.TrimSpace(pdfPath) == "" {
		return nil, nil, errors.New("path is required")
	}
	if runner == nil {
		return nil, nil, errors.New("pdf renderer is not configured")
	}
	if dpi == 0 {
		dpi = 200
	}
	workDir, err := os.MkdirTemp("", "aicli-pdf-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(workDir)
	}

	prefix := filepath.Join(workDir, "page")
	out, err := runner.CombinedOutput(ctx, tools.PDFToPPM, "-jpeg", "-r", itoa(dpi), pdfPath, prefix)
	if err != nil {
		cleanup()
		return nil, nil, errors.New(strings.TrimSpace(string(out)) + ": " + err.Error())
	}
	images, err := filepath.Glob(prefix + "-*.jpg")
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	sort.Strings(images)
	if len(images) == 0 {
		cleanup()
		return nil, nil, errors.New("no page images were produced")
	}
	return images, cleanup, nil
}

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
	if workers <= 0 {
		workers = 4
	}
	if workers > len(inputs) && len(inputs) > 0 {
		workers = len(inputs)
	}
	pages := make([]OCRPage, len(inputs))
	jobs := make(chan int)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				input := inputs[index]
				data := input.Data
				if data == nil {
					fileData, err := os.ReadFile(input.Path)
					if err != nil {
						sendErr(errCh, err, cancel)
						continue
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
					sendErr(errCh, err, cancel)
					continue
				}
				pages[index] = OCRPage{
					Name: input.Name,
					Path: input.Path,
					Text: strings.TrimSpace(res.Content),
				}
			}
		}()
	}
	for index := range inputs {
		select {
		case <-ctx.Done():
			break
		case jobs <- index:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	return pages, nil
}

func AssembleMarkdown(pages []OCRPage) string {
	parts := make([]string, 0, len(pages))
	for i, page := range pages {
		name := page.Name
		if name == "" {
			name = "page-" + itoa(i+1)
		}
		parts = append(parts, "<!-- Page "+itoa(i+1)+" "+name+" -->\n"+page.Text)
	}
	return strings.Join(parts, "\n\n---\n\n")
}

func sendErr(errCh chan error, err error, cancel context.CancelFunc) {
	select {
	case errCh <- err:
		cancel()
	default:
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
