package core

import (
	"encoding/json"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteError(w http.ResponseWriter, code int, err error) {
	WriteJSON(w, code, map[string]string{"error": err.Error()})
}
