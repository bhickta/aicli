package execution

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
)

func decodeImages(images []Image) ([]provider.VisionImage, error) {
	result := make([]provider.VisionImage, 0, len(images))
	for _, image := range images {
		encoded, mimeType := image.Data, image.MIMEType
		if strings.HasPrefix(encoded, "data:") {
			header, value, found := strings.Cut(encoded, ",")
			if !found {
				return nil, errors.New("invalid image data URL")
			}
			encoded = value
			mimeType = strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
		result = append(result, provider.VisionImage{Name: image.Name, Image: data, MIMEType: mimeType})
	}
	return result, nil
}
