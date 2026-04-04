package handlers

import (
	"errors"
	"lazymanga/models"
	"lazymanga/normalization"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type moveSingleRepoISORequest struct {
	TargetRepoID uint `json:"target_repo_id"`
}

// MoveSingleRepoISO moves one repoiso record and its ISO file into another repository.
func MoveSingleRepoISO(c *gin.Context) {
	repoID := strings.TrimSpace(c.Param("id"))
	isoID := strings.TrimSpace(c.Param("isoId"))
	if repoID == "" || isoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or isoId"})
		return
	}

	var req moveSingleRepoISORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.TargetRepoID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_repo_id required"})
		return
	}

	var sourceRepo models.Repository
	if err := db.First(&sourceRepo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query source repo failed: " + err.Error()})
		return
	}
	if sourceRepo.ID == req.TargetRepoID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target repo must be different from source repo"})
		return
	}

	sourceDB, sourceRootAbs, _, err := openRepoScopedDB(sourceRepo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare source repo db failed: " + err.Error()})
		return
	}

	sourceInfo, err := EnsureRepoInfoFromRepository(sourceDB, sourceRepo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load source repo info failed: " + err.Error()})
		return
	}
	if !sourceInfo.SingleMove {
		c.JSON(http.StatusForbidden, gin.H{"error": "single move is disabled for this repository"})
		return
	}

	var sourceRow models.RepoISO
	if err := sourceDB.First(&sourceRow, isoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source repo iso record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query source repo iso failed: " + err.Error()})
		return
	}

	var targetRepo models.Repository
	if err := db.First(&targetRepo, req.TargetRepoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "target repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query target repo failed: " + err.Error()})
		return
	}

	targetDB, targetRootAbs, _, err := openRepoScopedDB(targetRepo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare target repo db failed: " + err.Error()})
		return
	}

	targetInfo, err := EnsureRepoInfoFromRepository(targetDB, targetRepo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load target repo info failed: " + err.Error()})
		return
	}

	relPath := strings.TrimSpace(sourceRow.Path)
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source repo iso path is empty"})
		return
	}

	sourceAbs, err := resolveRepoISOAbsPath(sourceRootAbs, relPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resolve source path failed: " + err.Error()})
		return
	}
	targetAbs, err := resolveRepoISOAbsPath(targetRootAbs, relPath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "resolve target path failed: " + err.Error()})
		return
	}

	info, err := os.Stat(sourceAbs)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "source iso file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat source iso file failed: " + err.Error()})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source path is a directory"})
		return
	}

	var existingTargetRow models.RepoISO
	if err := targetDB.Where("path = ?", relPath).First(&existingTargetRow).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "target repo record already exists for same path", "target_repo_iso_id": existingTargetRow.ID})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query target repo record failed: " + err.Error()})
		return
	}

	if targetStat, err := os.Stat(targetAbs); err == nil {
		if !targetStat.IsDir() {
			c.JSON(http.StatusConflict, gin.H{"error": "target repo file already exists for same path"})
			return
		}
		c.JSON(http.StatusConflict, gin.H{"error": "target repo directory occupies same file path"})
		return
	} else if err != nil && !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stat target path failed: " + err.Error()})
		return
	}

	if err := os.MkdirAll(filepath.Dir(targetAbs), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare target folder failed: " + err.Error()})
		return
	}

	tx := targetDB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "begin target transaction failed: " + tx.Error.Error()})
		return
	}

	targetRow := sourceRow
	targetRow.ID = 0
	if err := tx.Create(&targetRow).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "insert target record failed: " + err.Error()})
		return
	}

	if err := moveRepoISOFileWithFallback(sourceAbs, targetAbs); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "move iso file failed: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		_ = moveRepoISOFileWithFallback(targetAbs, sourceAbs)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "commit target transaction failed: " + err.Error()})
		return
	}

	if err := sourceDB.Delete(&models.RepoISO{}, sourceRow.ID).Error; err != nil {
		_ = targetDB.Delete(&models.RepoISO{}, targetRow.ID).Error
		_ = moveRepoISOFileWithFallback(targetAbs, sourceAbs)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete source record failed: " + err.Error()})
		return
	}

	normalizeAsync := targetInfo.AutoNormalize
	if normalizeAsync {
		normalization.StartAsyncPostIndexNormalization(targetRepo.ID, targetDB, targetRootAbs, []models.RepoISO{targetRow})
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                   "single repo iso moved",
		"source_repo_id":            sourceRepo.ID,
		"target_repo_id":            targetRepo.ID,
		"source_repo_iso_id":        sourceRow.ID,
		"target_repo_iso_id":        targetRow.ID,
		"path":                      relPath,
		"target_auto_normalize":     targetInfo.AutoNormalize,
		"target_normalize_async":    normalizeAsync,
		"target_repo_iso":           targetRow,
		"single_move_source_enable": sourceInfo.SingleMove,
	})
}
