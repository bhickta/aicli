package document

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/bhickta/aicli/internal/config"
	"github.com/bhickta/aicli/internal/tool"
)

func RenderPDFToImages(ctx context.Context, tools config.ToolConfig, runner tool.Runner, pdfPath string, dpi int, workers int) ([]string, func(), error) {
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
	if countErr == nil && pages > 1 && workers != 1 {
		images, err := renderPDFPagesParallel(ctx, tools, runner, pdfPath, prefix, dpi, pages, workers)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		return images, cleanup, nil
	}
	if countErr != nil && workers > 1 {
		cleanup()
		return nil, nil, errors.New("pdfinfo is required for parallel PDF rendering: " + countErr.Error())
	}

	out, err := runner.CombinedOutput(ctx, tools.PDFToPPM, "-jpeg", "-r", strconv.Itoa(dpi), pdfPath, prefix)
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
	workers int,
) ([]string, error) {
	workers = normalizeWorkers(workers, pages)
	jobs := make(chan int)
	errCh := make(chan error, 1)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go renderPDFPageWorker(ctx, tools, runner, pdfPath, prefix, dpi, jobs, errCh, cancel, &wg)
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

func renderPDFPageWorker(
	ctx context.Context,
	tools config.ToolConfig,
	runner tool.Runner,
	pdfPath string,
	prefix string,
	dpi int,
	jobs <-chan int,
	errCh chan error,
	cancel context.CancelFunc,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for page := range jobs {
		pagePrefix := prefix + "-" + zeroPad(page)
		out, err := runner.CombinedOutput(
			ctx,
			tools.PDFToPPM,
			"-jpeg",
			"-r",
			strconv.Itoa(dpi),
			"-f",
			strconv.Itoa(page),
			"-l",
			strconv.Itoa(page),
			pdfPath,
			pagePrefix,
		)
		if err != nil {
			sendErr(errCh, commandError(out, err), cancel)
		}
	}
}

func normalizeWorkers(workers int, jobs int) int {
	if jobs <= 1 {
		return 1
	}
	if workers < 1 {
		return 1
	}
	if workers > jobs {
		return jobs
	}
	return workers
}

func commandError(out []byte, err error) error {
	message := strings.TrimSpace(string(out))
	if message == "" {
		return err
	}
	return errors.New(message + ": " + err.Error())
}

func sendErr(errCh chan error, err error, cancel context.CancelFunc) {
	select {
	case errCh <- err:
		cancel()
	default:
	}
}
