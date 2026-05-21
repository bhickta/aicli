package zettel

import "github.com/bhickta/aicli/internal/workflow/zettel/vaultfs"

type vault = vaultfs.Vault

var errOutsideVault = vaultfs.ErrOutsideVault

func newVault(path string) (vault, error) {
	return vaultfs.New(path)
}

func isInScope(rel string, options Options) bool {
	return vaultfs.IsInScope(rel, options)
}

func isMarkdown(path string) bool {
	return vaultfs.IsMarkdown(path)
}
