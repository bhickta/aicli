package ocr

import (
	"archive/zip"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bhickta/aicli/internal/workflow/document"
)

func zipImageInputs(path string) ([]document.ImageInput, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	files := imageFiles(reader.File)
	inputs := make([]document.ImageInput, 0, len(files))
	for _, file := range files {
		data, err := readZipFile(file)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, document.ImageInput{
			Name:     file.Name,
			Data:     data,
			MIMEType: mimeForName(file.Name),
		})
	}
	return inputs, nil
}

func imageFiles(files []*zip.File) []*zip.File {
	out := []*zip.File{}
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(file.Name))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" {
			out = append(out, file)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return naturalLess(out[i].Name, out[j].Name)
	})
	return out
}

func readZipFile(file *zip.File) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, 50<<20))
}

func mimeForName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func naturalLess(a string, b string) bool {
	return strings.ToLower(a) < strings.ToLower(b)
}
