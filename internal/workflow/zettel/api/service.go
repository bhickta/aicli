package zettel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/provider"
	"github.com/bhickta/aicli/internal/workflow/zettel/apicalls"
	"github.com/bhickta/aicli/internal/workflow/zettel/indexer"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

type Service struct {
	mergeProvider     provider.Provider
	embeddingProvider provider.Provider
	dataDir           string
}

func New(p provider.Provider) *Service {
	return NewWithEmbedding(p, p)
}

func NewWithEmbedding(p provider.Provider, embeddingProvider provider.Provider) *Service {
	return NewWithProviders(p, embeddingProvider)
}

func NewWithProviders(
	mergeProvider provider.Provider,
	embeddingProvider provider.Provider,
) *Service {
	return &Service{
		mergeProvider:     mergeProvider,
		embeddingProvider: embeddingProvider,
	}
}

func (s *Service) WithDataDir(dataDir string) *Service {
	copy := *s
	copy.dataDir = strings.TrimSpace(dataDir)
	return &copy
}

func (s *Service) Index(ctx context.Context, req IndexRequest, progress ProgressFunc) (IndexResponse, error) {
	options := s.workflowOptions(req.Options)
	v, err := vaultfs.New(options.VaultPath)
	if err != nil {
		return IndexResponse{}, err
	}
	tracker, _, embeddingProvider := s.trackedProviders()
	response, err := indexer.New(v, options, embeddingProvider).Build(ctx, progress)
	response.APICalls = tracker.Snapshot()
	return response, err
}

func (s *Service) workflowOptions(options Options) Options {
	options = normalizeOptions(options)
	if strings.TrimSpace(s.dataDir) == "" || filepath.IsAbs(options.DataFolder) {
		return options
	}
	options.DataFolder = centralZettelDataFolder(s.dataDir, options.VaultPath)
	return options
}

func centralZettelDataFolder(dataDir string, vaultPath string) string {
	vaultKey := strings.TrimSpace(vaultPath)
	if abs, err := filepath.Abs(vaultKey); err == nil {
		vaultKey = abs
	}
	sum := sha256.Sum256([]byte(vaultKey))
	hash := hex.EncodeToString(sum[:8])
	name := sanitizeDataFolderName(filepath.Base(vaultKey))
	return filepath.Join(dataDir, "zettel", name+"-"+hash)
}

func sanitizeDataFolderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "vault"
	}
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	return replacer.Replace(name)
}

func (s *Service) trackedProviders() (
	*apicalls.Tracker,
	provider.Provider,
	provider.Provider,
) {
	tracker := apicalls.NewTracker()
	return tracker,
		tracker.Wrap(s.mergeProvider),
		tracker.Wrap(s.embeddingProvider)
}
