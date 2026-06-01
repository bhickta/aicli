package training

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func exampleHash(example chatExample) string {
	return hashJSON(example)
}

func hashJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return hashBytes(data)
}

func hashText(value string) string {
	return hashBytes([]byte(value))
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
