package zettel

import "sort"

func ListNotes(options Options) (ListNotesResponse, error) {
	options = normalizeOptions(options)
	v, err := newVault(options.VaultPath)
	if err != nil {
		return ListNotesResponse{}, err
	}
	notes, err := v.ScanNotes(options)
	if err != nil {
		return ListNotesResponse{}, err
	}
	sort.Strings(notes)
	return ListNotesResponse{Notes: notes, Count: len(notes)}, nil
}
