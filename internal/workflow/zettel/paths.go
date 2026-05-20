package zettel

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var errOutsideVault = errors.New("path is outside vault")

type vault struct {
	root string
}

func newVault(path string) (vault, error) {
	if strings.TrimSpace(path) == "" {
		return vault{}, errors.New("vault path is required")
	}
	root, err := filepath.Abs(path)
	if err != nil {
		return vault{}, fmt.Errorf("resolve vault path: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return vault{}, fmt.Errorf("stat vault path: %w", err)
	}
	if !info.IsDir() {
		return vault{}, fmt.Errorf("vault path is not a directory: %s", root)
	}
	return vault{root: root}, nil
}

func (v vault) abs(relOrAbs string) (string, error) {
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
		return "", errOutsideVault
	}
	return abs, nil
}

func (v vault) rel(abs string) (string, error) {
	rel, err := filepath.Rel(v.root, abs)
	if err != nil {
		return "", fmt.Errorf("relativize path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", errOutsideVault
	}
	return filepath.ToSlash(rel), nil
}

func (v vault) dataPath(options Options, parts ...string) (string, error) {
	all := append([]string{options.DataFolder}, parts...)
	return v.abs(filepath.Join(all...))
}

func (v vault) notePath(rel string, options Options) (string, error) {
	if !isMarkdown(rel) {
		return "", fmt.Errorf("not a markdown note: %s", rel)
	}
	if !isInScope(rel, options) {
		return "", fmt.Errorf("note is outside zettelkasten scope: %s", rel)
	}
	return v.abs(rel)
}

func (v vault) scanNotes(options Options) ([]string, error) {
	root, err := v.abs(options.RootFolder)
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
		if !isMarkdown(path) {
			return nil
		}
		rel, err := v.rel(path)
		if err != nil {
			return err
		}
		if isInScope(rel, options) {
			notes = append(notes, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan notes: %w", err)
	}
	return notes, nil
}

func (v vault) scanInboxNotes(options Options) ([]string, error) {
	root, err := v.abs(options.InboxFolder)
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
		if !isMarkdown(path) {
			return nil
		}
		rel, err := v.rel(path)
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

func isInScope(rel string, options Options) bool {
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

func isMarkdown(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".md")
}
