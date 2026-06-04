package ocr

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func copyRenderedImages(dir string, images []string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(images))
	for index, image := range images {
		target := filepath.Join(dir, "page-"+zeroPad(index+1)+filepath.Ext(image))
		if err := copyFile(target, image); err != nil {
			return nil, err
		}
		out = append(out, target)
	}
	return out, nil
}

func copyFile(dst string, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func artifactURL(root string, path string) string {
	if root == "" || path == "" {
		return ""
	}
	rel, err := filepath.Rel(filepath.Dir(filepath.Dir(root)), path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return "/artifacts/" + strings.Join(parts, "/")
}

func zeroPad(number int) string {
	return fmt.Sprintf("%03d", number)
}
