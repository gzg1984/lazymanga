package handlers

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

func resolveInternalSourcePath(rawPath string) (string, string, error) {
	v := strings.TrimSpace(rawPath)
	if v == "" {
		return "", "", fmt.Errorf("empty path")
	}

	if strings.HasPrefix(strings.ToLower(v), "file://") {
		if u, err := url.Parse(v); err == nil {
			if p := strings.TrimSpace(u.Path); p != "" {
				v = p
			} else {
				v = strings.TrimPrefix(v, "file://")
			}
		} else {
			v = strings.TrimPrefix(v, "file://")
		}
	}

	normalized := strings.ReplaceAll(v, "\\", "/")
	clean := filepath.Clean(filepath.FromSlash(normalized))

	if filepath.IsAbs(clean) {
		if !isPathWithinRoot(internalRepoRoot, clean) {
			return "", "", fmt.Errorf("absolute path out of internal root")
		}
		rel, err := filepath.Rel(internalRepoRoot, clean)
		if err != nil {
			return "", "", err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		rel = strings.TrimPrefix(rel, "./")
		if rel == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			return "", "", fmt.Errorf("invalid relative path")
		}
		return rel, clean, nil
	}

	rel := filepath.ToSlash(filepath.Clean(clean))
	rel = strings.TrimPrefix(rel, "./")
	rel = strings.Trim(rel, "/")
	if rel == "" || rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf("invalid relative path")
	}

	abs := filepath.Join(internalRepoRoot, filepath.FromSlash(rel))
	if !isPathWithinRoot(internalRepoRoot, abs) {
		return "", "", fmt.Errorf("path out of internal root")
	}

	return rel, abs, nil
}
