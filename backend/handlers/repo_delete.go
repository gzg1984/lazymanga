package handlers

import (
	"errors"
	"io"
	"lazymanga/models"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type deleteRepoISORequest struct {
	DeleteFile bool `json:"delete_file"`
}

// DeleteMissingRepoISOs deletes all repoiso rows already marked as missing.
func DeleteMissingRepoISOs(c *gin.Context) {
	repoID := strings.TrimSpace(c.Param("id"))
	if repoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo failed: " + err.Error()})
		return
	}

	repoDB, _, _, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	result := repoDB.Where("is_missing = ?", true).Delete(&models.RepoISO{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete missing repository entries failed: " + result.Error.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "missing repository entries deleted",
		"repo_id":       repo.ID,
		"deleted_count": result.RowsAffected,
	})
}

// DeleteRepoISO deletes a repoiso row, optionally deleting the ISO file itself.
func DeleteRepoISO(c *gin.Context) {
	repoID := strings.TrimSpace(c.Param("id"))
	isoID := strings.TrimSpace(c.Param("isoId"))
	if repoID == "" || isoID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id or isoId"})
		return
	}

	var repo models.Repository
	if err := db.First(&repo, repoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo failed: " + err.Error()})
		return
	}

	repoDB, rootAbs, _, err := openRepoScopedDB(repo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "prepare repo db failed: " + err.Error()})
		return
	}

	info, err := EnsureRepoInfoFromRepository(repoDB, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "load repo info failed: " + err.Error()})
		return
	}

	var row models.RepoISO
	if err := repoDB.First(&row, isoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "repository entry not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repository entry failed: " + err.Error()})
		return
	}
	if !info.DeleteButton && !row.IsMissing {
		c.JSON(http.StatusForbidden, gin.H{"error": "delete button is disabled for this repository"})
		return
	}

	var req deleteRepoISORequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	fileExisted := false
	fileDeleted := false
	fileMissing := false
	if req.DeleteFile {
		absPath, err := resolveRepoISOAbsPath(rootAbs, row.Path)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source path: " + err.Error()})
			return
		}

		info, statErr := os.Stat(absPath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				fileMissing = true
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "stat managed path failed: " + statErr.Error()})
				return
			}
		} else {
			fileExisted = true
			var removeErr error
			if row.IsDirectory || info.IsDir() {
				removeErr = os.RemoveAll(absPath)
			} else {
				removeErr = os.Remove(absPath)
			}
			if removeErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "delete managed path failed: " + removeErr.Error()})
				return
			}
			fileDeleted = true
		}
	}

	if err := repoDB.Delete(&models.RepoISO{}, row.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete repository entry failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "repository entry deleted",
		"repo_id":        repo.ID,
		"iso_id":         row.ID,
		"path":           row.Path,
		"record_deleted": true,
		"delete_file":    req.DeleteFile,
		"file_existed":   fileExisted,
		"file_deleted":   fileDeleted,
		"file_missing":   fileMissing,
	})
}
