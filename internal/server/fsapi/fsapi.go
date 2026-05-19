package fsapi

type Handler struct {
	dataDir string
}

func New(dataDir string) *Handler {
	return &Handler{dataDir: dataDir}
}

type fileEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

type UploadEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}
