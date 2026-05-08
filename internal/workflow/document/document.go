package document

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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
	pages, countErr := pdfPageCount(ctx, runner, tools.PDFToPPM, pdfPath)
	if countErr == nil && pages > 1 {
		images, err := renderPDFPagesParallel(ctx, tools, runner, pdfPath, prefix, dpi, pages)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		return images, cleanup, nil
	}

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

func renderPDFPagesParallel(
	ctx context.Context,
	tools config.ToolConfig,
	runner tool.Runner,
	pdfPath string,
	prefix string,
	dpi int,
	pages int,
) ([]string, error) {
	workers := renderWorkers(pages, dpi)
	jobs := make(chan int)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for page := range jobs {
				pagePrefix := prefix + "-" + zeroPad(page)
				out, err := runner.CombinedOutput(
					ctx,
					tools.PDFToPPM,
					"-jpeg",
					"-r",
					itoa(dpi),
					"-f",
					itoa(page),
					"-l",
					itoa(page),
					pdfPath,
					pagePrefix,
				)
				if err != nil {
					message := strings.TrimSpace(string(out))
					if message == "" {
						message = err.Error()
					} else {
						message += ": " + err.Error()
					}
					sendErr(errCh, errors.New(message), cancel)
				}
			}
		}()
	}
sendPages:
	for page := 1; page <= pages; page++ {
		select {
		case <-ctx.Done():
			break sendPages
		case jobs <- page:
		}
	}
	close(jobs)
	wg.Wait()

	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	images, err := filepath.Glob(prefix + "-*.jpg")
	if err != nil {
		return nil, err
	}
	sort.Strings(images)
	if len(images) == 0 {
		return nil, errors.New("no page images were produced")
	}
	return images, nil
}

func pdfPageCount(ctx context.Context, runner tool.Runner, pdfToPPM string, pdfPath string) (int, error) {
	out, err := runner.CombinedOutput(ctx, pdfInfoCommand(pdfToPPM), pdfPath)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.EqualFold(strings.TrimSuffix(fields[0], ":"), "Pages") {
			pages, err := strconv.Atoi(fields[1])
			if err != nil {
				return 0, err
			}
			if pages <= 0 {
				return 0, errors.New("pdf has no pages")
			}
			return pages, nil
		}
	}
	return 0, errors.New("pdf page count not found")
}

func pdfInfoCommand(pdfToPPM string) string {
	if dir := filepath.Dir(pdfToPPM); dir != "." && dir != "" {
		return filepath.Join(dir, "pdfinfo")
	}
	return "pdfinfo"
}

func renderWorkers(pages int, dpi int) int {
	if pages <= 1 {
		return 1
	}
	cpuWorkers := runtime.NumCPU() / 2
	if cpuWorkers < 1 {
		cpuWorkers = 1
	}
	if cpuWorkers > 4 {
		cpuWorkers = 4
	}

	workers := minInt(cpuWorkers, pages)
	if memoryWorkers := renderWorkersByMemory(dpi); memoryWorkers > 0 {
		workers = minInt(workers, memoryWorkers)
	}
	if workers < 1 {
		return 1
	}
	return workers
}

func renderWorkersByMemory(dpi int) int {
	available, ok := availableMemoryBytes()
	if !ok {
		return 0
	}
	perPage := estimatedRenderBytes(dpi)
	if perPage <= 0 {
		return 0
	}
	workers := int(available / perPage)
	if workers < 1 {
		return 1
	}
	return workers
}

func estimatedRenderBytes(dpi int) uint64 {
	if dpi <= 0 {
		dpi = 200
	}
	// A4-ish page, RGB pixels, plus headroom for poppler buffers.
	width := uint64(9 * dpi)
	height := uint64(12 * dpi)
	return width * height * 3 * 2
}

func availableMemoryBytes() (uint64, bool) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func zeroPad(value int) string {
	if value < 10 {
		return "000" + itoa(value)
	}
	if value < 100 {
		return "00" + itoa(value)
	}
	if value < 1000 {
		return "0" + itoa(value)
	}
	return itoa(value)
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
		workers = 1
	}
	if workers > len(inputs) && len(inputs) > 0 {
		workers = len(inputs)
	}
	pages := make([]OCRPage, len(inputs))
	jobs := make(chan int)
	errCh := make(chan pageError, len(inputs))

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
						errCh <- pageError{Name: input.Name, Err: err}
						pages[index] = failedOCRPage(input, err)
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
					errCh <- pageError{Name: input.Name, Err: err}
					pages[index] = failedOCRPage(input, err)
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
