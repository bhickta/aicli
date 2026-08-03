package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	geminiKeysFilename = "gemini-keys.json"
	maxGeminiKeysBytes = 1 << 20
	maxGeminiLaneKeys  = 32
)

type geminiKeyRecord struct {
	Key    string `json:"key"`
	Status string `json:"status"`
}

func loadConfiguredGeminiLaneKeys(configuredPath string) error {
	path := strings.TrimSpace(configuredPath)
	missingOK := path == ""
	if path == "" {
		var err error
		path, err = defaultGeminiKeysPath()
		if err != nil {
			return err
		}
	}
	_, err := loadGeminiLaneKeys(path, missingOK)
	return err
}

func defaultGeminiKeysPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(configDir, "aicli", geminiKeysFilename), nil
}

func loadGeminiLaneKeys(path string, missingOK bool) (int, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && missingOK {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect key file: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, errors.New("key file must not be a symbolic link")
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("key file must be a regular file")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return 0, errors.New("key file permissions must not grant group or other access")
	}
	if info.Size() > maxGeminiKeysBytes {
		return 0, errors.New("key file exceeds the 1 MiB limit")
	}

	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open key file: %w", err)
	}
	defer file.Close()
	openedInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("inspect opened key file: %w", err)
	}
	if !os.SameFile(info, openedInfo) {
		return 0, errors.New("key file changed while it was being opened")
	}
	if !openedInfo.Mode().IsRegular() || openedInfo.Mode().Perm()&0o077 != 0 {
		return 0, errors.New("opened key file is not a private regular file")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxGeminiKeysBytes+1))
	if err != nil {
		return 0, fmt.Errorf("read key file: %w", err)
	}
	if len(data) > maxGeminiKeysBytes {
		return 0, errors.New("key file exceeds the 1 MiB limit")
	}

	var records []geminiKeyRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return 0, fmt.Errorf("parse key file JSON: %w", err)
	}
	if len(records) > maxGeminiLaneKeys {
		return 0, fmt.Errorf("key file contains more than %d records", maxGeminiLaneKeys)
	}

	validKeys := make([]string, 0, len(records))
	seen := make(map[[sha256.Size]byte]struct{}, len(records))
	for _, record := range records {
		if !strings.EqualFold(strings.TrimSpace(record.Status), "valid") {
			continue
		}
		key := record.Key
		if key == "" || key != strings.TrimSpace(key) {
			return 0, errors.New("valid key records must contain a non-empty, trimmed key")
		}
		fingerprint := sha256.Sum256([]byte(key))
		if _, exists := seen[fingerprint]; exists {
			return 0, errors.New("key file contains a duplicate valid key")
		}
		seen[fingerprint] = struct{}{}
		validKeys = append(validKeys, key)
	}
	if len(validKeys) == 0 {
		return 0, errors.New("key file contains no valid keys")
	}

	for index, key := range validKeys {
		name := fmt.Sprintf("GEMINI_LANE_%d_KEY", index+1)
		if os.Getenv(name) != "" {
			continue
		}
		if err := os.Setenv(name, key); err != nil {
			return 0, fmt.Errorf("set lane %d environment: %w", index+1, err)
		}
	}
	return len(validKeys), nil
}
