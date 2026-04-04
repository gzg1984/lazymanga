package handlers

import (
	"fmt"
	"lazymanga/models"
	"os"
	"path/filepath"
	"strings"
)

const maxMergeFailureMessages = 30

// RepoMergeFlowResult describes merge-transfer main flow execution details.
type RepoMergeFlowResult struct {
	SourceRepoID            uint     `json:"source_repo_id"`
	TargetRepoID            uint     `json:"target_repo_id"`
	SourceRootPath          string   `json:"source_root_path"`
	TargetRootPath          string   `json:"target_root_path"`
	SourceTotal             int      `json:"source_total"`
	Processed               int      `json:"processed"`
	Merged                  int      `json:"merged"`
	SkippedRecordExists     int      `json:"skipped_record_exists"`
	SkippedTargetFileExists int      `json:"skipped_target_file_exists"`
	SkippedSourceMissing    int      `json:"skipped_source_missing"`
	Failed                  int      `json:"failed"`
	SourceRemaining         int64    `json:"source_remaining"`
	Failures                []string `json:"failures,omitempty"`
}

// RepoMergeProgress describes realtime progress state for merge flow execution.
type RepoMergeProgress struct {
	Processed   int    `json:"processed"`
	Total       int    `json:"total"`
	CurrentPath string `json:"current_path"`
	CurrentStep string `json:"current_step"`
}

type RepoMergeProgressCallback func(progress RepoMergeProgress)

// ExecuteRepoMergeFlow runs the main merge flow by comparing source rows against target rows.
func ExecuteRepoMergeFlow(sourceRepo models.Repository, targetRepo models.Repository) (RepoMergeFlowResult, error) {
	return ExecuteRepoMergeFlowWithProgress(sourceRepo, targetRepo, nil)
}

// ExecuteRepoMergeFlowWithProgress runs merge flow and reports realtime progress via callback.
func ExecuteRepoMergeFlowWithProgress(sourceRepo models.Repository, targetRepo models.Repository, progressCb RepoMergeProgressCallback) (RepoMergeFlowResult, error) {
	result := RepoMergeFlowResult{
		SourceRepoID: sourceRepo.ID,
		TargetRepoID: targetRepo.ID,
		Failures:     make([]string, 0),
	}

	report := func(step string, currentPath string) {
		if progressCb == nil {
			return
		}
		progressCb(RepoMergeProgress{
			Processed:   result.Processed,
			Total:       result.SourceTotal,
			CurrentPath: currentPath,
			CurrentStep: step,
		})
	}

	markRecordDone := func(step string, currentPath string) {
		result.Processed++
		report(step, currentPath)
	}

	sourceDB, sourceRootAbs, _, err := openRepoScopedDB(sourceRepo)
	if err != nil {
		return result, fmt.Errorf("open source repo db failed: %w", err)
	}
	targetDB, targetRootAbs, _, err := openRepoScopedDB(targetRepo)
	if err != nil {
		return result, fmt.Errorf("open target repo db failed: %w", err)
	}

	result.SourceRootPath = sourceRootAbs
	result.TargetRootPath = targetRootAbs

	var sourceRows []models.RepoISO
	if err := sourceDB.Order("id asc").Find(&sourceRows).Error; err != nil {
		return result, fmt.Errorf("query source repoisos failed: %w", err)
	}
	result.SourceTotal = len(sourceRows)
	report("start", "")

	var targetRows []models.RepoISO
	if err := targetDB.Select("id", "path").Find(&targetRows).Error; err != nil {
		return result, fmt.Errorf("query target repoisos failed: %w", err)
	}
	targetPathSet := make(map[string]struct{}, len(targetRows))
	for _, row := range targetRows {
		targetPathSet[row.Path] = struct{}{}
	}

	for _, sourceRow := range sourceRows {
		relPath := strings.TrimSpace(sourceRow.Path)
		report("checking", relPath)
		if relPath == "" {
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("source row id=%d has empty path", sourceRow.ID))
			markRecordDone("failed-empty-path", relPath)
			continue
		}

		if _, exists := targetPathSet[relPath]; exists {
			result.SkippedRecordExists++
			markRecordDone("skipped-record-exists", relPath)
			continue
		}

		sourceAbs, err := resolveRepoISOAbsPath(sourceRootAbs, relPath)
		if err != nil {
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("resolve source path failed id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-resolve-source", relPath)
			continue
		}
		targetAbs, err := resolveRepoISOAbsPath(targetRootAbs, relPath)
		if err != nil {
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("resolve target path failed id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-resolve-target", relPath)
			continue
		}

		sourceInfo, err := os.Stat(sourceAbs)
		if err != nil {
			if os.IsNotExist(err) {
				result.SkippedSourceMissing++
				markRecordDone("skipped-source-missing", relPath)
				continue
			}
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("stat source failed id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-stat-source", relPath)
			continue
		}
		if sourceInfo.IsDir() {
			result.SkippedSourceMissing++
			markRecordDone("skipped-source-is-dir", relPath)
			continue
		}

		targetInfo, err := os.Stat(targetAbs)
		if err == nil {
			if !targetInfo.IsDir() {
				result.SkippedTargetFileExists++
				markRecordDone("skipped-target-file-exists", relPath)
				continue
			}
			result.SkippedTargetFileExists++
			markRecordDone("skipped-target-dir-exists", relPath)
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("stat target failed id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-stat-target", relPath)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("mkdir target dir failed id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-mkdir-target", relPath)
			continue
		}

		tx := targetDB.Begin()
		if tx.Error != nil {
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("begin target transaction failed id=%d error=%v", sourceRow.ID, tx.Error))
			markRecordDone("failed-begin-target-tx", relPath)
			continue
		}

		targetRow := sourceRow
		targetRow.ID = 0
		if err := tx.Create(&targetRow).Error; err != nil {
			tx.Rollback()
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("insert target row failed source_id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-insert-target-row", relPath)
			continue
		}

		report("moving-file", relPath)

		if err := moveRepoISOFileWithFallback(sourceAbs, targetAbs); err != nil {
			tx.Rollback()
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("move source file failed source_id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-move-file", relPath)
			continue
		}

		// Guarantee source file is removed even if underlying filesystem behavior is unusual.
		if _, err := os.Stat(sourceAbs); err == nil {
			if err := os.Remove(sourceAbs); err != nil {
				_ = moveRepoISOFileWithFallback(targetAbs, sourceAbs)
				tx.Rollback()
				result.Failed++
				appendMergeFailure(&result, fmt.Sprintf("delete source file failed source_id=%d path=%q error=%v", sourceRow.ID, relPath, err))
				markRecordDone("failed-delete-source-file", relPath)
				continue
			}
		} else if err != nil && !os.IsNotExist(err) {
			_ = moveRepoISOFileWithFallback(targetAbs, sourceAbs)
			tx.Rollback()
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("recheck source file failed source_id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-recheck-source-file", relPath)
			continue
		}

		if err := tx.Commit().Error; err != nil {
			_ = moveRepoISOFileWithFallback(targetAbs, sourceAbs)
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("commit target transaction failed source_id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-commit-target-tx", relPath)
			continue
		}

		if err := sourceDB.Delete(&models.RepoISO{}, sourceRow.ID).Error; err != nil {
			_ = targetDB.Delete(&models.RepoISO{}, targetRow.ID).Error
			_ = moveRepoISOFileWithFallback(targetAbs, sourceAbs)
			result.Failed++
			appendMergeFailure(&result, fmt.Sprintf("delete source row failed source_id=%d path=%q error=%v", sourceRow.ID, relPath, err))
			markRecordDone("failed-delete-source-row", relPath)
			continue
		}

		targetPathSet[relPath] = struct{}{}
		result.Merged++
		markRecordDone("merged", relPath)
	}

	if err := sourceDB.Model(&models.RepoISO{}).Count(&result.SourceRemaining).Error; err != nil {
		return result, fmt.Errorf("count source remaining rows failed: %w", err)
	}
	report("done", "")

	return result, nil
}

func appendMergeFailure(result *RepoMergeFlowResult, message string) {
	if len(result.Failures) >= maxMergeFailureMessages {
		return
	}
	result.Failures = append(result.Failures, message)
}
