package document

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/tool"
)

func PDFPageCount(ctx context.Context, runner tool.Runner, pdfToPPM string, pdfPath string) (int, error) {
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

func SplitPDFRange(ctx context.Context, runner tool.Runner, qpdf string, sourcePath string, outputPath string, firstPage int, lastPage int) error {
	if firstPage <= 0 || lastPage < firstPage {
		return fmt.Errorf("invalid PDF page range %d-%d", firstPage, lastPage)
	}
	qpdf = strings.TrimSpace(qpdf)
	if qpdf == "" {
		qpdf = "qpdf"
	}
	out, err := runner.CombinedOutput(
		ctx,
		qpdf,
		sourcePath,
		"--pages",
		".",
		fmt.Sprintf("%d-%d", firstPage, lastPage),
		"--",
		outputPath,
	)
	if err != nil {
		return fmt.Errorf("split PDF pages %d-%d: %w: %s", firstPage, lastPage, err, tool.LimitedOutput(out, 2000))
	}
	return nil
}

func pdfInfoCommand(pdfToPPM string) string {
	if dir := filepath.Dir(pdfToPPM); dir != "." && dir != "" {
		return filepath.Join(dir, "pdfinfo")
	}
	return "pdfinfo"
}
