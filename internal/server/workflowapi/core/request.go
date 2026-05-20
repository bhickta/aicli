package core

import (
	"encoding/json"
	"net/http"

	"github.com/bhickta/aicli/internal/provider"
)

func DecodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var req T
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, err)
		return req, false
	}
	return req, true
}

func (r *Runtime) ProviderOrError(w http.ResponseWriter, id string) (provider.Provider, bool) {
	p, ok := r.ProviderFor(id)
	if !ok {
		WriteError(w, http.StatusNotFound, ErrProviderNotFound)
		return nil, false
	}
	return p, true
}
