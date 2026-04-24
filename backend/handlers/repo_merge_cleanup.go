package handlers

import (
	"errors"
	"io"
	"lazymanga/models"
	"os"
)

// RepoMergeCleanupResult describes post-merge source repo cleanup result.
type RepoMergeCleanupResult struct {
	SourceRepoID        uint   `json:"source_repo_id"`
	SourceRepoBasic     bool   `json:"source_repo_basic"`
	SourceRemaining     int64  `json:"source_remaining"`
	SourceRepoProtected bool   `json:"source_repo_protected"`
	SourceRepoDeleted   bool   `json:"source_repo_deleted"`
	SourceDBFileDeleted bool   `json:"source_db_file_deleted"`
	SourceRootRemoved   bool   `json:"source_root_removed"`
	Message             string `json:"message"`
}

// ExecuteRepoMergeCleanup removes source repo metadata when source rows are fully consumed.
func ExecuteRepoMergeCleanup(sourceRepo models.Repository) (RepoMergeCleanupResult, error) {
	result := RepoMergeCleanupResult{SourceRepoID: sourceRepo.ID, SourceRepoBasic: sourceRepo.Basic}

	sourceDB, sourceRootAbs, sourceDBPath, err := openRepoScopedDB(sourceRepo)
	if err != nil {
		return result, err
	}

	if err := sourceDB.Model(&models.RepoISO{}).Count(&result.SourceRemaining).Error; err != nil {
		return result, err
	}
	if result.SourceRemaining > 0 {
		result.Message = "source repo retained because some records remain"
		return result, nil
	}

	if sourceRepo.Basic {
		result.SourceRepoProtected = true
		result.Message = "source repo retained because it is a basic repository"
		return result, nil
	}

	if err := db.Delete(&models.Repository{}, sourceRepo.ID).Error; err != nil {
		return result, err
	}
	result.SourceRepoDeleted = true

	if err := os.Remove(sourceDBPath); err == nil {
		result.SourceDBFileDeleted = true
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, err
	}

	empty, err := isDirectoryEmpty(sourceRootAbs)
	if err != nil {
		return result, err
	}
	if empty {
		if err := os.Remove(sourceRootAbs); err == nil {
			result.SourceRootRemoved = true
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return result, err
		}
	}

	result.Message = "source repo deleted after merge"

	return result, nil
}

func isDirectoryEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, nil
		}
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}
