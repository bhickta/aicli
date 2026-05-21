package vaultfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bhickta/aicli/internal/workflow/zettel/model"
)

var ErrOutsideVault = errors.New("path is outside vault")

type Vault struct {
	root string
}

func New(path string) (Vault, error) {
	if strings.TrimSpace(path) == "" {
		return Vault{}, errors.New("vault path is required")
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return Vault{}, fmt.Errorf("resolve vault path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return Vault{}, fmt.Errorf("stat vault path: %w", err)
	}
	if !info.IsDir() {
		return Vault{}, fmt.Errorf("vault path is not a directory: %s", root)
	}
	return Vault{root: root}, nil
}

func (v Vault) Abs(relOrAbs string) (string, error) {
	if strings.TrimSpace(relOrAbs) == "" {
		return "", errors.New("path is required")
	}
	path := relOrAbs
	if !filepath.IsAbs(path) {
		path = filepath.Join(v.root, path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}
	rel, err := filepath.Rel(v.root, abs)
	if err != nil {
		return "", fmt.Errorf("relativize path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ErrOutsideVault
	}
	return abs, nil
}

func (v Vault) Rel(abs string) (string, error) {
	rel, err := filepath.Rel(v.root, abs)
	if err != nil {
		return "", fmt.Errorf("relativize path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ErrOutsideVault
	}
	return filepath.ToSlash(rel), nil
}

func (v Vault) DataPath(options model.Options, parts ...string) (string, error) {
	all := append([]string{options.DataFolder}, parts...)
	return v.Abs(filepath.Join(all...))
}

func (v Vault) NotePath(rel string, options model.Options) (string, error) {
	if !IsMarkdown(rel) {
		return "", fmt.Errorf("not a markdown note: %s", rel)
	}
	if !IsInScope(rel, options) {
		return "", fmt.Errorf("note is outside zettelkasten scope: %s", rel)
	}
	return v.Abs(rel)
}

func (v Vault) ScanNotes(options model.Options) ([]string, error) {
	root, err := v.Abs(options.RootFolder)
	if err != nil {
		return nil, err
	}
	var notes []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsMarkdown(path) {
			return nil
		}
		rel, err := v.Rel(path)
		if err != nil {
			return err
		}
		if IsInScope(rel, options) {
			notes = append(notes, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan notes: %w", err)
	}
	return notes, nil
}

func (v Vault) ScanInboxNotes(options model.Options) ([]string, error) {
	root, err := v.Abs(options.InboxFolder)
	if err != nil {
		return nil, err
	}
	var notes []string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, ".") && path != root {
				return filepath.SkipDir
			}
			if name == "_processed" || name == "_pending" {
				return filepath.SkipDir
			}
			return nil
		}
		if !IsMarkdown(path) {
			return nil
		}
		rel, err := v.Rel(path)
		if err != nil {
			return err
		}
		notes = append(notes, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan inbox notes: %w", err)
	}
	return notes, nil
}

func IsInScope(rel string, options model.Options) bool {
	clean := strings.Trim(filepath.ToSlash(filepath.Clean(rel)), "/")
	root := strings.Trim(filepath.ToSlash(filepath.Clean(options.RootFolder)), "/")
	data := strings.Trim(filepath.ToSlash(filepath.Clean(options.DataFolder)), "/")
	inbox := strings.Trim(filepath.ToSlash(filepath.Clean(options.InboxFolder)), "/")
	if root == "." || root == "" {
		return false
	}
	if data != "." && data != "" && (clean == data || strings.HasPrefix(clean, data+"/")) {
		return false
	}
	if inbox != "." && inbox != "" && (clean == inbox || strings.HasPrefix(clean, inbox+"/")) {
		return false
	}
	return clean == root || strings.HasPrefix(clean, root+"/")
}

func IsMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}
