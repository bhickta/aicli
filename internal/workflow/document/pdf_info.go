package document

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bhickta/aicli/internal/tool"
)

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
