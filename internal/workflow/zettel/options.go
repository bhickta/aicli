package zettel

import "github.com/bhickta/aicli/internal/workflow/zettel/model"

func NormalizeOptions(options Options) Options {
	return normalizeOptions(options)
}

func normalizeOptions(options Options) Options {
	return model.NormalizeOptions(options)
}
