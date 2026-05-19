package document

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
