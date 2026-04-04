package handlers

import (
	"fmt"
	"lazyiso/models"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openRepoScopedDB(repo models.Repository) (*gorm.DB, string, string, error) {
	rootAbs, err := resolveRepoRootAbs(repo)
	if err != nil {
		return nil, "", "", err
	}

	dbFile := strings.TrimSpace(repo.DBFile)
	if dbFile == "" {
		dbFile = "repo.db"
	}
	// Keep db file name sanitized; path resolution differs for basic vs non-basic.
	dbFile = filepath.Base(dbFile)

	dbBaseDir := rootAbs
	boundaryRoot := rootAbs
	if repo.Basic {
		dbBaseDir = filepath.Clean(filepath.FromSlash(strings.TrimSpace(basicRepoDBDir)))
		boundaryRoot = dbBaseDir
		if dbBaseDir == "" || !filepath.IsAbs(dbBaseDir) {
			return nil, "", "", fmt.Errorf("invalid basic repo db dir")
		}
	}

	dbPath := filepath.Join(dbBaseDir, dbFile)
	if !isPathWithinRoot(boundaryRoot, dbPath) {
		return nil, "", "", fmt.Errorf("db file out of boundary")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, "", "", fmt.Errorf("prepare repo db directory failed: %w", err)
	}

	// Ensure db file exists so first-time normalize can always materialize repo db.
	if _, err := os.Stat(dbPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, "", "", fmt.Errorf("stat repo db file failed: %w", err)
		}
		f, createErr := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o644)
		if createErr != nil {
			return nil, "", "", fmt.Errorf("create repo db file failed: %w", createErr)
		}
		if closeErr := f.Close(); closeErr != nil {
			return nil, "", "", fmt.Errorf("close repo db file failed: %w", closeErr)
		}
	}

	repoDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, "", "", fmt.Errorf("open repo db failed: %w", err)
	}

	if err := repoDB.AutoMigrate(&models.RepoInfo{}, &models.RepoISO{}); err != nil {
		return nil, "", "", fmt.Errorf("migrate repo tables failed: %w", err)
	}

	return repoDB, rootAbs, dbPath, nil
}

func resolveRepoRootAbs(repo models.Repository) (string, error) {
	if repo.Basic {
		return resolveBasicRepoRootAbs(repo.RootPath)
	}

	baseRoot := internalRepoRoot
	boundaryRoot := internalRepoRoot

	if !repo.IsInternal {
		deviceName, err := normalizeExternalDeviceName(repo.ExternalDeviceName)
		if err != nil {
			return "", fmt.Errorf("invalid external device: %w", err)
		}
		baseRoot = filepath.Join(externalRepoRoot, filepath.FromSlash(deviceName))
		boundaryRoot = externalRepoRoot
	}

	normalizedRootPath, err := normalizeRepoRootPath(repo.RootPath, false)
	if err != nil {
		return "", fmt.Errorf("invalid repo root_path: %w", err)
	}

	rootAbs := baseRoot
	if normalizedRootPath != "/" {
		rootAbs = filepath.Join(baseRoot, filepath.FromSlash(normalizedRootPath))
	}
	if !isPathWithinRoot(boundaryRoot, rootAbs) {
		return "", fmt.Errorf("repo root out of boundary")
	}

	info, err := os.Stat(rootAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("repo root not found")
		}
		return "", fmt.Errorf("stat repo root failed: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repo root is not a directory")
	}

	return rootAbs, nil
}

func resolveBasicRepoRootAbs(rootPath string) (string, error) {
	v := strings.TrimSpace(strings.ReplaceAll(rootPath, "\\", "/"))
	if v == "" {
		return "", fmt.Errorf("basic repo root_path required")
	}

	rootAbs := filepath.Clean(filepath.FromSlash(v))
	if !filepath.IsAbs(rootAbs) {
		return "", fmt.Errorf("basic repo root_path must be absolute")
	}

	info, err := os.Stat(rootAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("basic repo root not found")
		}
		return "", fmt.Errorf("stat basic repo root failed: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("basic repo root is not a directory")
	}

	return rootAbs, nil
}
