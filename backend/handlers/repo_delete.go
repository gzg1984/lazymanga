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
			c.JSON(http.StatusNotFound, gin.H{"error": "repo iso record not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "query repo iso failed: " + err.Error()})
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

		if _, err := os.Stat(absPath); err != nil {
			if os.IsNotExist(err) {
				fileMissing = true
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "stat repo iso file failed: " + err.Error()})
				return
			}
		} else {
			fileExisted = true
			if err := os.Remove(absPath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "delete repo iso file failed: " + err.Error()})
				return
			}
			fileDeleted = true
		}
	}

	if err := repoDB.Delete(&models.RepoISO{}, row.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete repo iso record failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "repo iso deleted",
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
