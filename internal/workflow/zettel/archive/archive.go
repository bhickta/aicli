package archive

import (
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
	"github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"
)

type Store struct {
	vault   vaultfs.Vault
	options model.Options
}

func NewStore(v vaultfs.Vault, options model.Options) Store {
	return Store{vault: v, options: options}
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "note"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return replacer.Replace(name)
}
